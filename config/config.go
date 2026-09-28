package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// Config holds all runtime configuration for the service, sourced from
// environment variables.
type Config struct {
	Port             string
	DBURL            string
	RedisURL         string
	JWTPublicKeyPath string
	MinioEndpoint    string
	MinioAccessKey   string
	MinioSecretKey   string
	MinioUseSSL      bool
}

// defaultJWTPublicKeyPath points at cert/public_key.pem, resolved via this
// file's own location (not the process's working directory) so `go run .`
// works the same regardless of the caller's current directory.
var defaultJWTPublicKeyPath = func() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "..", "cert", "public_key.pem")
}()

// Load reads configuration from environment variables. It does not validate
// values beyond applying defaults for PORT and JWT_PUBLIC_KEY_PATH; callers
// should fail fast if a required value (e.g. DBURL) is empty.
func Load() *Config {
	return &Config{
		Port:             getEnv("PORT", "8080"),
		DBURL:            os.Getenv("DB_URL"),
		RedisURL:         os.Getenv("REDIS_URL"),
		JWTPublicKeyPath: getEnv("JWT_PUBLIC_KEY_PATH", defaultJWTPublicKeyPath),
		MinioEndpoint:    getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey:   getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey:   getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioUseSSL:      getEnvBool("MINIO_USE_SSL", false),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
