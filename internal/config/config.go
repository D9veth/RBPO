package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Address, DatabaseURL, AppURL, Secret, StaticDir string
	SecureCookies                                   bool
}

func Load() (Config, error) {
	c := Config{Address: value("HTTP_ADDR", ":8080"), DatabaseURL: os.Getenv("DATABASE_URL"), AppURL: value("APP_URL", "http://localhost:8080"), Secret: os.Getenv("APP_SECRET"), StaticDir: value("STATIC_DIR", "dist")}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("DATABASE_URL is required")
	}
	if len(c.Secret) < 32 {
		return c, fmt.Errorf("APP_SECRET must contain at least 32 characters")
	}
	u, err := url.Parse(c.AppURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || u.User != nil || (u.Path != "" && u.Path != "/") {
		return c, fmt.Errorf("APP_URL must be a valid origin")
	}
	c.AppURL = strings.TrimRight(c.AppURL, "/")
	c.SecureCookies = u.Scheme == "https"
	if !c.SecureCookies && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1" {
		return c, fmt.Errorf("non-local deployments require HTTPS")
	}
	return c, nil
}
func value(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
