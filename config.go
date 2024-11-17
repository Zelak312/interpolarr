package main

import (
	"errors"
	"os"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

type Config struct {
	BindAddress                 string        `yaml:"bindAddress"`
	Port                        int32         `yaml:"port"`
	RifeBinary                  string        `yaml:"rifeBinary"`
	ProcessFolder               string        `yaml:"processFolder"`
	DatabasePath                string        `yaml:"databasePath"`
	LogPath                     string        `yaml:"logPath"`
	ModelPath                   string        `yaml:"modelPath"`
	Workers                     int           `yaml:"workers"`
	TargetFPS                   float64       `yaml:"targetFPS"`
	FfmpegOptions               FfmpegOptions `yaml:"ffmpegOptions"`
	DeleteInputFileWhenFinished *bool         `yaml:"deleteInputFileWhenFinished"`
	DeleteOutputIfAlreadyExist  *bool         `yaml:"deleteOutputIfAlreadyExist"`
	CopyFileToDestinationOnSkip *bool         `yaml:"copyFileToDestinationOnSkip"`
	RifeExtraArguments          string        `yaml:"rifeExtraArguments"`
}

type FfmpegOptions struct {
	HWAccelDecodeFlag string `yaml:"HWAccelDecodeFlag"`
	HWAccelEncodeFlag string `yaml:"HWAccelEncodeFlag"`
}

// Verify config and set defaults
func verifyConfig(config *Config) error {
	if config == nil {
		return errors.New("cannot verify config, config is nil")
	}

	if config.BindAddress == "" {
		config.BindAddress = "127.0.0.1"
	}

	if config.Port == 0 {
		config.Port = 80
	}

	if config.RifeBinary == "" {
		return errors.New("missing rife binary path in config")
	}

	if config.ProcessFolder == "" {
		return errors.New("missing temp process folder in config")
	}

	if config.DatabasePath == "" {
		return errors.New("missing database path in config")
	}

	if config.ModelPath == "" {
		config.ModelPath = "rife-v4.7"
	}

	if config.Workers == 0 {
		config.Workers = 1
	}

	if config.TargetFPS == 0 {
		config.TargetFPS = 60
	}

	if config.DeleteInputFileWhenFinished == nil {
		defaultVal := false
		config.DeleteInputFileWhenFinished = &defaultVal
	}

	if config.DeleteOutputIfAlreadyExist == nil {
		defaultVal := false
		config.DeleteOutputIfAlreadyExist = &defaultVal
	}

	if config.CopyFileToDestinationOnSkip == nil {
		defaultVal := false
		config.CopyFileToDestinationOnSkip = &defaultVal
	}

	if config.LogPath == "" {
		config.LogPath = "./logs"
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

	// Override with env variables if they are passed in
	err = envconfig.ProcessWithOptions("", &config, envconfig.Options{SplitWords: true})
	if err != nil {
		return Config{}, err
	}

	err = verifyConfig(&config)
	if err != nil {
		return Config{}, err
	}

	return config, nil
}
