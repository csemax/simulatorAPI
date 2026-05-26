package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppAddr           string
	DanteBaseURL     string
	LegacyBaseURL    string
	HTTPClientTimeout time.Duration
}

func Load() Config {
	timeoutSeconds := getEnvInt("HTTP_CLIENT_TIMEOUT_SECONDS", 30)

	return Config{
		AppAddr:            getEnv("APP_ADDR", ":8090"),
		DanteBaseURL:      normalizeBaseURL(getEnv("DANTE_BASE_URL", "http://localhost:8080")),
		LegacyBaseURL:     normalizeBaseURL(getEnv("LEGACY_BASE_URL", "https://legacy.litegral.com")),
		HTTPClientTimeout: time.Duration(timeoutSeconds) * time.Second,
	}
}

func getEnv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func normalizeBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}