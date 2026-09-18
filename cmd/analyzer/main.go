// Команда analyzer — интерактивная оболочка универсального глубокого анализатора Bybit.
package main

import (
	"bufio"
	"context"
	"crypto-coin-analyzer/internal/analysis"
	"crypto-coin-analyzer/internal/bybit"
	"crypto-coin-analyzer/internal/output"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var validSymbol = regexp.MustCompile(`^[A-Z0-9]{2,30}(USDT|USDC)$`)

// getExeDir возвращает директорию бинарника, чтобы папка reports находилась рядом с программой.
func getExeDir() string {
	p, e := os.Executable()
	if e != nil {
		return "."
	}
	if p2, e := filepath.EvalSymlinks(p); e == nil {
		p = p2
	}
	return filepath.Dir(p)
}

// normalizeSymbol приводит сокращение BTC к BTCUSDT, но сохраняет уже полный BTCUSDT/BTCUSDC.
func normalizeSymbol(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	if !strings.HasSuffix(s, "USDT") && !strings.HasSuffix(s, "USDC") {
		s += "USDT"
	}
	return s
}

// parseSymbols проверяет формат, удаляет дубликаты и жёстко ограничивает сеанс десятью символами.
func parseSymbols(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("пустой ввод")
	}
	if strings.ContainsAny(s, " \t") {
		return nil, fmt.Errorf("используйте запятые без пробелов, например BTC,ETH,SOL")
	}
	parts := strings.Split(s, ",")
	if len(parts) > 10 {
		return nil, fmt.Errorf("за один сеанс разрешено максимум 10 символов")
	}
	seen := map[string]bool{}
	out := []string{}
	for _, p := range parts {
		x := normalizeSymbol(p)
		if !validSymbol.MatchString(x) {
			return nil, fmt.Errorf("некорректный символ: %q", p)
		}
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out, nil
}

// askChoice показывает числовое меню. Пустой Enter возвращает defaultValue.
func askChoice(r *bufio.Reader, title string, options []string, defaultValue int) int {
	for {
		fmt.Println("\n" + title)
		for i, o := range options {
			suffix := ""
			if i+1 == defaultValue {
				suffix = " [по умолчанию]"
			}
			fmt.Printf("%d — %s%s\n", i+1, o, suffix)
		}
		fmt.Printf("Ваш выбор [%d]: ", defaultValue)
		s, _ := r.ReadString('\n')
		s = strings.TrimSpace(s)
		if s == "" {
			return defaultValue
		}
		for i := range options {
			if s == fmt.Sprint(i+1) {
				return i + 1
			}
		}
		fmt.Println("Некорректный выбор. Повторите ввод.")
	}
}

// chooseRequest получает тип анализа и направление. Оба меню по Enter выбирают максимально полный режим №3.
func chooseRequest(r *bufio.Reader) analysis.Request {
	a := askChoice(r, "Что анализируем?", []string{"GRID BOT", "Обычная позиция (Directional)", "Полный анализ (GRID + Directional)"}, 3)
	d := askChoice(r, "Какое направление анализируем?", []string{"LONG", "SHORT", "LONG + SHORT"}, 3)
	at := []string{"grid", "directional", "full"}[a-1]
	dir := []string{"long", "short", "both"}[d-1]
	return analysis.Request{AnalysisType: at, Direction: dir}
}

// saveReport создаёт отдельный timestamped JSON для каждой монеты.
func saveReport(dir, symbol string, r analysis.Report) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s_%s.json", symbol, r.GeneratedAt.Format("20060102_150405Z"))
	p := filepath.Join(dir, name)
	f, e := os.Create(p)
	if e != nil {
		return "", e
	}
	defer f.Close()
	if e = output.WriteJSON(f, r, true); e != nil {
		return "", e
	}
	return p, nil
}

// analyzeSession сначала проверяет существование символов через лёгкий ticker-запрос, затем запускает глубокий сбор только для валидных инструментов.
func analyzeSession(reader *bufio.Reader, client *bybit.Client, outDir string) {
	req := chooseRequest(reader)
	var symbols []string
	for {
		fmt.Print("\nВведите 1-10 символов через запятую БЕЗ пробелов (BTC,ETH,1000PEPE): ")
		line, _ := reader.ReadString('\n')
		v, e := parseSymbols(line)
		if e != nil {
			fmt.Println("[ОШИБКА]", e)
			continue
		}
		symbols = v
		break
	}
	fmt.Println("\nПроверка символов...")
	valid := []string{}
	for _, s := range symbols {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		_, e := client.Ticker(ctx, s)
		cancel()
		if e != nil {
			fmt.Printf("✗ %s — инструмент не найден или Bybit недоступен: %v\n", s, e)
			continue
		}
		fmt.Printf("✓ %s\n", s)
		valid = append(valid, s)
	}
	if len(valid) == 0 {
		fmt.Println("Нет валидных символов для анализа.")
		return
	}
	start := time.Now()
	ok := 0
	for i, s := range valid {
		fmt.Printf("[%d/%d] %s — глубокий анализ...\n", i+1, len(valid), s)
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		report, e := analysis.Build(ctx, client, s, req)
		cancel()
		if e != nil {
			fmt.Printf("  [ОШИБКА] %v\n", e)
			continue
		}
		p, e := saveReport(outDir, s, report)
		if e != nil {
			fmt.Printf("  [ОШИБКА ЗАПИСИ] %v\n", e)
			continue
		}
		ok++
		fmt.Printf("  [ГОТОВО] %s | regime=%s | grid=%s | directional long=%s | short=%s\n", p, report.MarketRegime.Classification, report.Grid.Regime, report.Directional.Long.State, report.Directional.Short.State)
	}
	fmt.Printf("\nАнализ завершён. Успешно: %d, ошибок: %d, время: %s\n", ok, len(valid)-ok, time.Since(start).Round(time.Second))
}

// askContinue после Enter запускает новый полный диалог; 0 завершает программу.
func askContinue(r *bufio.Reader) bool {
	for {
		fmt.Print("\nЧто дальше?\n1 — Новый анализ [по умолчанию]\n0 — Выход\nВаш выбор [1]: ")
		s, _ := r.ReadString('\n')
		s = strings.TrimSpace(s)
		if s == "" || s == "1" {
			return true
		}
		if s == "0" {
			return false
		}
		fmt.Println("Введите 1 или 0.")
	}
}

// main создаёт публичный Bybit V5 клиент и обслуживает последовательные интерактивные сеансы.
func main() {
	log.SetFlags(0)
	reader := bufio.NewReader(os.Stdin)
	client := bybit.NewClient(bybit.Config{})
	outDir := filepath.Join(getExeDir(), "reports")
	fmt.Println("====================================================")
	fmt.Println(" BYBIT UNIVERSAL DEEP COIN ANALYZER v3")
	fmt.Println("====================================================")
	fmt.Println("Публичные данные Bybit V5. API-ключ не нужен.")
	fmt.Println("Raw evidence сохраняется всегда. JSON-отчёты:", outDir)
	for {
		analyzeSession(reader, client, outDir)
		if !askContinue(reader) {
			fmt.Println("Работа завершена.")
			break
		}
	}
}
