// Package config loads and validates runtime configuration from environment
// variables. Required values fail fast at startup instead of falling back to
// hidden defaults.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultAllowedExtensions is used when ALLOWED_EXTENSIONS is not set.
const DefaultAllowedExtensions = "pdf,doc,docx,odt,txt,md,rtf,ppt,pptx,odp,xls,xlsx,csv,ods,png,jpg,jpeg,gif,webp,zip,7z,tar,gz"

// Config is the complete runtime configuration of the service.
type Config struct {
	Addr           string
	DBPath         string
	MaxUploadBytes int64 // whole request
	MaxFileBytes   int64 // single file
	AllowedExt     map[string]bool
	SessionTTL     time.Duration
	CookieSecure   bool
	ShareMaxTTL    time.Duration
	Admin          Admin
	S3             S3
}

// Admin is the bootstrap administrator created on first start.
type Admin struct {
	Name     string
	Email    string
	Password string
}

// S3 holds the object storage connection settings.
type S3 struct {
	Endpoint       string
	PublicEndpoint string // address browsers use for share links; empty disables sharing
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	UsePathStyle   bool
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
	positive := func(key, fallback string) int64 {
		n, err := strconv.ParseInt(optional(key, fallback), 10, 64)
		if err != nil || n <= 0 {
			errs = append(errs, fmt.Errorf("%s must be a positive integer", key))
		}
		return n
	}
	boolean := func(key, fallback string) bool {
		b, err := strconv.ParseBool(optional(key, fallback))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s must be true or false", key))
		}
		return b
	}
	endpoint := func(key, v string) string {
		if v == "" {
			return ""
		}
		u, err := url.Parse(v)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, fmt.Errorf("%s must be an http(s) URL such as http://rustfs:9000", key))
		}
		return strings.TrimRight(v, "/")
	}

	cfg := Config{
		Addr:           optional("APP_ADDR", ":8000"),
		DBPath:         optional("DB_PATH", "data/file-storage.db"),
		MaxUploadBytes: positive("MAX_UPLOAD_MB", "64") << 20,
		MaxFileBytes:   positive("MAX_FILE_MB", "25") << 20,
		SessionTTL:     time.Duration(positive("SESSION_TTL_HOURS", "12")) * time.Hour,
		CookieSecure:   boolean("COOKIE_SECURE", "false"),
		ShareMaxTTL:    time.Duration(positive("SHARE_MAX_HOURS", "168")) * time.Hour,
		Admin: Admin{
			Name:     optional("ADMIN_NAME", "Administrator"),
			Email:    required("ADMIN_EMAIL"),
			Password: required("ADMIN_PASSWORD"),
		},
		S3: S3{
			Endpoint:     endpoint("S3_ENDPOINT", required("S3_ENDPOINT")),
			Bucket:       required("S3_BUCKET"),
			AccessKey:    required("S3_ACCESS_KEY"),
			SecretKey:    required("S3_SECRET_KEY"),
			Region:       optional("S3_REGION", "us-east-1"),
			UsePathStyle: boolean("S3_USE_PATH_STYLE", "true"),
		},
	}
	cfg.S3.PublicEndpoint = endpoint("S3_PUBLIC_ENDPOINT", strings.TrimSpace(getenv("S3_PUBLIC_ENDPOINT")))

	cfg.AllowedExt = map[string]bool{}
	for _, ext := range strings.Split(optional("ALLOWED_EXTENSIONS", DefaultAllowedExtensions), ",") {
		if ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), ".")); ext != "" {
			cfg.AllowedExt[ext] = true
		}
	}
	if len(cfg.AllowedExt) == 0 {
		errs = append(errs, errors.New("ALLOWED_EXTENSIONS must list at least one extension"))
	}
	if len(cfg.Admin.Password) > 0 && len(cfg.Admin.Password) < 8 {
		errs = append(errs, errors.New("ADMIN_PASSWORD must be at least 8 characters"))
	}
	if cfg.MaxFileBytes > cfg.MaxUploadBytes {
		errs = append(errs, errors.New("MAX_FILE_MB cannot exceed MAX_UPLOAD_MB"))
	}
	if cfg.ShareMaxTTL > 168*time.Hour {
		errs = append(errs, errors.New("SHARE_MAX_HOURS cannot exceed 168 (7 days, the S3 presign limit)"))
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("invalid configuration:\n%w", err)
	}
	return cfg, nil
}
