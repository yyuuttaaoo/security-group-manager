package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Log    LogConfig    `yaml:"log"`
	Auth   AuthConfig   `yaml:"auth"`
	Alipay AlipayConfig `yaml:"alipay"`
	Baidu  BaiduConfig  `yaml:"baidu"`
	Server ServerConfig `yaml:"server"`
}

type AlipayConfig struct {
	AppID                string `yaml:"app_id"`
	PrivateKeyPath       string `yaml:"private_key_path"`
	AppCertPath          string `yaml:"app_cert_path"`
	AlipayPublicCertPath string `yaml:"alipay_public_cert_path"`
	AlipayRootCertPath   string `yaml:"alipay_root_cert_path"`
	RedirectURI          string `yaml:"redirect_uri"`
}

type BaiduConfig struct {
	AppID       string `yaml:"app_id"`
	AppKey      string `yaml:"app_key"`
	SecretKey   string `yaml:"secret_key"`
	SignKey     string `yaml:"sign_key"`
	RedirectURI string `yaml:"redirect_uri"`
}

type ServerConfig struct {
	Address  string `yaml:"address"` // e.g., "127.0.0.1" or "0.0.0.0"
	Port     string `yaml:"port"`
	TLS      bool   `yaml:"tls"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	DevMode  bool   `yaml:"dev_mode"`
}

type LogConfig struct {
	Output     string `yaml:"output"` // "stdout" or "file"
	FilePath   string `yaml:"file_path"`
	MaxSize    int    `yaml:"max_size"` // megabytes
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"` // days
	Compress   bool   `yaml:"compress"`
}

type AuthConfig struct {
	Enabled       bool         `yaml:"enabled"`
	SessionSecret string       `yaml:"session_secret"`
	CookieSecure  bool         `yaml:"cookie_secure"`
	Users         []UserConfig `yaml:"users"`
}

type UserConfig struct {
	UID            string   `yaml:"uid"`
	AlipayUserID   string   `yaml:"alipay_userid"`
	BaiduOpenID    string   `yaml:"baidu_openid"`
	LegacyUsername string   `yaml:"username,omitempty"`
	Groups         []string `yaml:"groups"`
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
