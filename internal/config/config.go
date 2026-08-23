package config

import (
	"encoding/json"
	"os"
)

type ByBit struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Secret  string `json:"secret"`
}

type Config struct {
	ByBit ByBit `json:"bybit"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var conf Config
	err = json.Unmarshal(data, &conf)
	if err != nil {
		return Config{}, err
	}
	return conf, nil
}
