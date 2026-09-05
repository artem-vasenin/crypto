package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"crypto-coin-analyzer/internal/analysis"
	"crypto-coin-analyzer/internal/bybit"
	"crypto-coin-analyzer/internal/output"
)

// getExeDir возвращает абсолютный путь к директории, в которой лежит скомпилированный бинарник
func getExeDir() string {
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("[WARN] Не удалось определить путь к бинарнику, используем '.': %v", err)
		return "."
	}
	// EvalSymlinks резолвит символические ссылки, если файл запущен через symlink
	resolvedPath, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		resolvedPath = exePath
	}
	return filepath.Dir(resolvedPath)
}

func main() {
	var (
		symbolFlag = flag.String("symbol", "", "Символ Bybit USDT Perpetual (например: SOLUSDT)")
		outFlag    = flag.String("out", "", "Путь к JSON файлу (если пусто — вывод в stdout)")
		daysFlag   = flag.Int("days", 7, "Глубина анализа в днях (1-30)")
		prettyFlag = flag.Bool("pretty", true, "Форматировать JSON с отступами")
	)
	flag.Parse()

	symbol := strings.ToUpper(strings.TrimSpace(*symbolFlag))
	outPath := strings.TrimSpace(*outFlag)
	days := *daysFlag
	pretty := *prettyFlag

	if symbol == "" {
		symbol, days, outPath, pretty = runInteractiveMenu()
	} else {
		symbol = sanitizeSymbol(symbol)
		// Если флаг -out передан относительно (без явного пути), кладем его рядом с бинарником
		if outPath != "" && !filepath.IsAbs(outPath) {
			outPath = filepath.Join(getExeDir(), outPath)
		}
	}

	if days < 1 {
		days = 1
	}
	if days > 30 {
		days = 30
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	client := bybit.NewClient(bybit.Config{
		BaseURL: "https://api.bybit.com",
		Timeout: 20 * time.Second,
	})

	log.Printf("[INFO] Запуск анализа для %s (глубина: %d дн.)...", symbol, days)
	result, err := analysis.Build(ctx, client, symbol, days)
	if err != nil {
		log.Fatalf("[ERROR] Ошибка анализа %s: %v", symbol, err)
	}

	if outPath == "" {
		if err := output.WriteJSON(os.Stdout, result, pretty); err != nil {
			log.Fatalf("[ERROR] Ошибка записи в stdout: %v", err)
		}
		return
	}

	f, err := os.Create(outPath)
	if err != nil {
		log.Fatalf("[ERROR] Не удалось создать файл %s: %v", outPath, err)
	}
	defer f.Close()

	if err := output.WriteJSON(f, result, pretty); err != nil {
		log.Fatalf("[ERROR] Не удалось записать JSON в %s: %v", outPath, err)
	}

	fmt.Fprintf(os.Stderr, "\n[SUCCESS] Успешно! Результат сохранен в файл: %s\n", outPath)
}

func runInteractiveMenu() (string, int, string, bool) {
	reader := bufio.NewReader(os.Stdin)
	exeDir := getExeDir()

	fmt.Println("==================================================")
	fmt.Println("    CRYPTO COIN ANALYZER — ИНТЕРАКТИВНОЕ МЕНЮ     ")
	fmt.Println("==================================================")

	var symbol string
	for {
		fmt.Print("1. Введите тикер монеты (например: SOL, BTCUSDT): ")
		input, _ := reader.ReadString('\n')
		symbol = sanitizeSymbol(input)
		if symbol != "" {
			break
		}
		fmt.Println("   [!] Тикер не может быть пустым. Повторите ввод.")
	}

	days := 7
	for {
		fmt.Print("2. Глубина анализа в днях [1-30] (по умолчанию 7): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			break
		}
		val, err := strconv.Atoi(input)
		if err == nil && val >= 1 && val <= 30 {
			days = val
			break
		}
		fmt.Println("   [!] Введите корректное число от 1 до 30.")
	}

	// Дефолтный путь формируется внутри папки бинарника
	defaultFileName := fmt.Sprintf("%s.json", symbol)
	defaultFullPath := filepath.Join(exeDir, defaultFileName)

	fmt.Printf("3. Файл для сохранения JSON (Enter для '%s', '-' для вывода на экран): ", defaultFullPath)
	outPath, _ := reader.ReadString('\n')
	outPath = strings.TrimSpace(outPath)

	if outPath == "" {
		outPath = defaultFullPath
	} else if outPath == "-" {
		outPath = ""
	} else if !filepath.IsAbs(outPath) {
		// Если пользователь ввел пользовательское относительное имя, тоже кладем его в папку бинарника
		outPath = filepath.Join(exeDir, outPath)
	}

	pretty := true
	fmt.Print("4. Форматировать JSON красивыми отступами? (Y/n): ")
	prettyInput, _ := reader.ReadString('\n')
	prettyInput = strings.ToLower(strings.TrimSpace(prettyInput))
	if prettyInput == "n" || prettyInput == "no" {
		pretty = false
	}

	fmt.Println("--------------------------------------------------")
	return symbol, days, outPath, pretty
}

func sanitizeSymbol(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	if !strings.HasSuffix(s, "USDT") && !strings.HasSuffix(s, "USDC") {
		s += "USDT"
	}
	return s
}
