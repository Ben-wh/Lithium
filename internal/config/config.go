// Package config loads runtime configuration from environment variables.
package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Addr          string // listen address, e.g. ":8080"
	DataDir       string // directory for the database and attachments
	DBPath        string // SQLite database file
	SecureCookies bool   // set the Secure flag on cookies (enable behind HTTPS)
	TrustProxy    bool   // trust X-Forwarded-For for the client IP
	AdminEmail    string // bootstrap admin, created only when no users exist
	AdminPassword string
	AdminName     string
}

func Load() Config {
	dataDir := env("LITHIUM_DATA_DIR", "./data")
	return Config{
		Addr:          env("LITHIUM_ADDR", ":8080"),
		DataDir:       dataDir,
		DBPath:        env("LITHIUM_DB_PATH", filepath.Join(dataDir, "lithium.db")),
		SecureCookies: envBool("LITHIUM_SECURE_COOKIES", false),
		TrustProxy:    envBool("LITHIUM_TRUST_PROXY", false),
		AdminEmail:    os.Getenv("LITHIUM_ADMIN_EMAIL"),
		AdminPassword: os.Getenv("LITHIUM_ADMIN_PASSWORD"),
		AdminName:     env("LITHIUM_ADMIN_NAME", "Administrator"),
	}
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return def
	}
	return v
}
