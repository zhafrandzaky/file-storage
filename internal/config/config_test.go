package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadRequiresS3Settings(t *testing.T) {
	_, err := load(env(map[string]string{}))
	if err == nil {
		t.Fatal("expected error for missing required variables")
	}
	for _, key := range []string{"S3_ENDPOINT", "S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error should mention %s, got: %v", key, err)
		}
	}
}

func TestLoadAppliesOptionalDefaults(t *testing.T) {
	cfg, err := load(env(map[string]string{
		"S3_ENDPOINT":   "http://rustfs:9000",
		"S3_BUCKET":     "docs",
		"S3_ACCESS_KEY": "a",
		"S3_SECRET_KEY": "b",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8000" || cfg.S3.Region != "us-east-1" || !cfg.S3.UsePathStyle || cfg.MaxUploadBytes != 64<<20 {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadRejectsInvalidUploadLimit(t *testing.T) {
	_, err := load(env(map[string]string{
		"S3_ENDPOINT": "x", "S3_BUCKET": "x", "S3_ACCESS_KEY": "x", "S3_SECRET_KEY": "x",
		"MAX_UPLOAD_MB": "0",
	}))
	if err == nil || !strings.Contains(err.Error(), "MAX_UPLOAD_MB") {
		t.Fatalf("expected MAX_UPLOAD_MB error, got %v", err)
	}
}
