package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Log LogConfig `yaml:"log"`
}

type LogConfig struct {
	Output     string `yaml:"output"` // "stdout" or "file"
	FilePath   string `yaml:"file_path"`
	MaxSize    int    `yaml:"max_size"` // megabytes
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"` // days
	Compress   bool   `yaml:"compress"`
}

func LoadConfig(path string) (*Config, error) {
	config := &Config{
		Log: LogConfig{
			Output: "stdout", // Default
		},
	}

	file, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil // Return default if file not found
		}
		return nil, err
	}

	err = yaml.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
