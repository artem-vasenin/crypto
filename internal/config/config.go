package config

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type ByBit struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"-"`
	Secret  string `json:"-"`
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

	if conf.ByBit.Secret == "" || conf.ByBit.APIKey == "" {
		return Config{}, errors.New("missing secret key")
	}

	return conf, nil
}
