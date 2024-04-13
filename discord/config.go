package main

import (
	"encoding/json"
	"os"
)

type DiscordConfig struct {
	Discord_token string `json:"Discord_token"`
	App_id        string `json:"App_id"`
	Public_key    string `json:"Public_key"`
	// Add more fields as needed
}

func readConfig(filename string) (DiscordConfig, error) {
	var config DiscordConfig

	// Read JSON file
	data, err := os.ReadFile(filename)
	if err != nil {
		return config, err
	}

	// Unmarshal JSON data into Config struct
	err = json.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}
