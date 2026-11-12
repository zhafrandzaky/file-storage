package config

import (
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func base() map[string]string {
	return map[string]string{
		"S3_ENDPOINT":    "http://rustfs:9000",
		"S3_BUCKET":      "files",
		"S3_ACCESS_KEY":  "a",
		"S3_SECRET_KEY":  "b",
		"ADMIN_EMAIL":    "admin@example.com",
		"ADMIN_PASSWORD": "long-enough",
	}
}

func TestLoadRequiresSettings(t *testing.T) {
	_, err := load(env(map[string]string{}))
	if err == nil {
		t.Fatal("expected error for missing required variables")
	}
	for _, key := range []string{"S3_ENDPOINT", "S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY", "ADMIN_EMAIL", "ADMIN_PASSWORD"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error should mention %s, got: %v", key, err)
		}
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := load(env(base()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8000" || cfg.S3.Region != "us-east-1" || !cfg.S3.UsePathStyle {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.MaxUploadBytes != 64<<20 || cfg.MaxFileBytes != 25<<20 || cfg.SessionTTL != 12*time.Hour {
		t.Errorf("unexpected limits: %+v", cfg)
	}
	if !cfg.AllowedExt["pdf"] || cfg.AllowedExt["exe"] {
		t.Errorf("unexpected extensions: %v", cfg.AllowedExt)
	}
	if cfg.S3.PublicEndpoint != "" {
		t.Errorf("share should be disabled by default")
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	cases := map[string]string{
		"MAX_UPLOAD_MB":      "0",
		"ADMIN_PASSWORD":     "short",
		"S3_PUBLIC_ENDPOINT": "rustfs:9000",
		"SHARE_MAX_HOURS":    "500",
		"COOKIE_SECURE":      "maybe",
	}
	for key, value := range cases {
		m := base()
		m[key] = value
		if _, err := load(env(m)); err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("%s=%q: expected error mentioning it, got %v", key, value, err)
		}
	}
}
