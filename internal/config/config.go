// Package config loads and validates runtime configuration from environment
// variables. Required values fail fast at startup instead of falling back to
// hidden defaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is the complete runtime configuration of the service.
type Config struct {
	Addr           string
	MaxUploadBytes int64
	S3             S3
}

// S3 holds the object storage connection settings.
type S3 struct {
	Endpoint     string
	Region       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
}

// Load reads configuration from the process environment.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	var errs []error

	required := func(key string) string {
		v := strings.TrimSpace(getenv(key))
		if v == "" {
			errs = append(errs, fmt.Errorf("%s is required", key))
		}
		return v
	}
	optional := func(key, fallback string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return fallback
	}

	cfg := Config{
		Addr: optional("APP_ADDR", ":8000"),
		S3: S3{
			Endpoint:  required("S3_ENDPOINT"),
			Bucket:    required("S3_BUCKET"),
			AccessKey: required("S3_ACCESS_KEY"),
			SecretKey: required("S3_SECRET_KEY"),
			Region:    optional("S3_REGION", "us-east-1"),
		},
	}

	pathStyle, err := strconv.ParseBool(optional("S3_USE_PATH_STYLE", "true"))
	if err != nil {
		errs = append(errs, fmt.Errorf("S3_USE_PATH_STYLE must be true or false: %w", err))
	}
	cfg.S3.UsePathStyle = pathStyle

	maxMB, err := strconv.ParseInt(optional("MAX_UPLOAD_MB", "64"), 10, 64)
	if err != nil || maxMB <= 0 {
		errs = append(errs, errors.New("MAX_UPLOAD_MB must be a positive integer"))
	}
	cfg.MaxUploadBytes = maxMB << 20

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("invalid configuration:\n%w", err)
	}
	return cfg, nil
}
