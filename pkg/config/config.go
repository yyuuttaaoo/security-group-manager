package config

import (
	"fmt"
	"os"
	"strings"

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

func (c *Config) Validate() error {
	if c.Auth.Enabled {
		if isPlaceholder(c.Auth.SessionSecret) || len(c.Auth.SessionSecret) < 32 {
			return fmt.Errorf("auth.session_secret must be a non-placeholder value with at least 32 characters")
		}
	}

	if alipayConfigured(c.Alipay) {
		required := map[string]string{
			"alipay.app_id":                  c.Alipay.AppID,
			"alipay.private_key_path":        c.Alipay.PrivateKeyPath,
			"alipay.app_cert_path":           c.Alipay.AppCertPath,
			"alipay.alipay_public_cert_path": c.Alipay.AlipayPublicCertPath,
			"alipay.alipay_root_cert_path":   c.Alipay.AlipayRootCertPath,
			"alipay.redirect_uri":            c.Alipay.RedirectURI,
		}
		if err := validateRequiredNoPlaceholders(required); err != nil {
			return err
		}
	}

	if baiduConfigured(c.Baidu) {
		required := map[string]string{
			"baidu.app_key":      c.Baidu.AppKey,
			"baidu.secret_key":   c.Baidu.SecretKey,
			"baidu.redirect_uri": c.Baidu.RedirectURI,
		}
		if err := validateRequiredNoPlaceholders(required); err != nil {
			return err
		}
	}

	return nil
}

func alipayConfigured(cfg AlipayConfig) bool {
	return cfg.AppID != "" || cfg.PrivateKeyPath != "" || cfg.AppCertPath != "" || cfg.AlipayPublicCertPath != "" || cfg.AlipayRootCertPath != "" || cfg.RedirectURI != ""
}

func baiduConfigured(cfg BaiduConfig) bool {
	return cfg.AppID != "" || cfg.AppKey != "" || cfg.SecretKey != "" || cfg.RedirectURI != ""
}

func validateRequiredNoPlaceholders(values map[string]string) error {
	for name, value := range values {
		if value == "" {
			return fmt.Errorf("%s is required when provider is configured", name)
		}
		if isPlaceholder(value) {
			return fmt.Errorf("%s must not be a placeholder", name)
		}
	}
	return nil
}

func isPlaceholder(value string) bool {
	upper := strings.ToUpper(strings.TrimSpace(value))
	return upper == "" ||
		strings.Contains(upper, "CHANGE_ME") ||
		strings.Contains(upper, "YOUR_") ||
		strings.Contains(upper, "EXAMPLE.COM")
}
