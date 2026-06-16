package config

import (
	"strings"
	"testing"
)

func TestValidateRejectsPlaceholderSessionSecret(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{
			Enabled:       true,
			SessionSecret: "CHANGE_ME_RANDOM_32_BYTES_OR_MORE",
		},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "session_secret") {
		t.Fatalf("Validate err = %v, want session_secret placeholder rejection", err)
	}
}

func TestValidateAllowsDisabledAuthWithoutSecret(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
}

func TestValidateRejectsProviderPlaceholders(t *testing.T) {
	cfg := &Config{
		Alipay: AlipayConfig{
			AppID:                "YOUR_ALIPAY_APP_ID",
			PrivateKeyPath:       "cert/app_private_key.txt",
			AppCertPath:          "cert/app_cert.crt",
			AlipayPublicCertPath: "cert/alipay_cert.crt",
			AlipayRootCertPath:   "cert/alipay_root_cert.crt",
			RedirectURI:          "https://your-domain.example/api/oauth/alipay/callback",
		},
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("Validate err = %v, want placeholder rejection", err)
	}
}
