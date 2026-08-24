package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// ByBit содержит настройки подключения к Bybit.
//
// Здесь находятся именно настройки инфраструктуры.
//
// API key и secret мы намеренно НЕ храним в JSON.
// Они приходят из переменных окружения.
//
// Это важно, потому что config.json можно спокойно хранить
// в Git, а секреты туда попадать не должны.
type ByBit struct {
	BaseURL             string `json:"base_url"`
	PrivateWebSocketURL string `json:"private_websocket_url"`

	APIKey string `json:"-"`
	Secret string `json:"-"`
}

// Config — корневая конфигурация приложения.
type Config struct {
	ByBit ByBit `json:"bybit"`
}

// Load загружает конфигурацию из JSON.
//
// Дополнительно пытаемся загрузить .env.
//
// Если .env отсутствует — это НЕ ошибка.
//
// Например, на production-сервере переменные могут быть
// установлены непосредственно в окружении ОС.
func Load(path string) (Config, error) {
	// godotenv.Load() пытается загрузить файл .env.
	//
	// Если файла нет — продолжаем работу.
	err := godotenv.Load()

	if err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	// Читаем JSON-файл.
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var conf Config

	// Преобразуем JSON в Go-структуру.
	err = json.Unmarshal(data, &conf)
	if err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	// API key и secret берём из окружения.
	conf.ByBit.APIKey = os.Getenv("BYBIT_API_KEY")
	conf.ByBit.Secret = os.Getenv("BYBIT_API_SECRET")

	// Проверяем обязательные параметры.
	if conf.ByBit.BaseURL == "" {
		return Config{}, errors.New("missing bybit base_url")
	}

	if conf.ByBit.PrivateWebSocketURL == "" {
		return Config{}, errors.New(
			"missing bybit private_websocket_url",
		)
	}

	if conf.ByBit.APIKey == "" {
		return Config{}, errors.New("missing BYBIT_API_KEY")
	}

	if conf.ByBit.Secret == "" {
		return Config{}, errors.New("missing BYBIT_API_SECRET")
	}

	return conf, nil
}
