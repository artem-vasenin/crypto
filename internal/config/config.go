package config

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type ByBit struct {
	BaseURL             string `json:"base_url"`
	PrivateWebSocketURL string `json:"private_ws_url"`
	APIKey              string `json:"-"`
	Secret              string `json:"-"`
}

type Config struct {
	ByBit ByBit `json:"bybit"`
}

func Load(path string) (Config, error) {
	err := godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var conf Config

	err = json.Unmarshal(data, &conf)
	if err != nil {
		return Config{}, err
	}

	conf.ByBit.APIKey = os.Getenv("BYBIT_API_KEY")
	conf.ByBit.Secret = os.Getenv("BYBIT_API_SECRET")

	if conf.ByBit.BaseURL == "" {
		return Config{}, errors.New("missing Bybit base URL")
	}

	if conf.ByBit.PrivateWebSocketURL == "" {
		return Config{}, errors.New("missing Bybit private WebSocket URL")
	}

	if conf.ByBit.APIKey == "" {
		return Config{}, errors.New("missing Bybit API key")
	}

	if conf.ByBit.Secret == "" {
		return Config{}, errors.New("missing Bybit API secret")
	}

	return conf, nil
}
