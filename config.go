package main

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BindAddress string `yaml:"bindAddress"`
	Port        int32  `yaml:"port"`
}

func verifyConfig(config *Config) error {
	if config == nil {
		return errors.New("cannot verify config, config is nil")
	}

	if config.BindAddress == "" {
		config.BindAddress = "127.0.0.1"
	}

	if config.Port == 0 {
		config.Port = 8090
	}

	return nil
}

func GetConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	config := Config{}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return Config{}, err
	}

	err = verifyConfig(&config)
	if err != nil {
		return Config{}, err
	}

	return config, nil
}
