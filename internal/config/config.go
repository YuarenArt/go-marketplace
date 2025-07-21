package config

import (
	"flag"
	"os"
)

// Config содержит настройки сервера и базы данных
type Config struct {
	Port      string
	JWTSecret string
	DB        DBConfig
}

// DBConfig содержит параметры подключения к PostgreSQL
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// NewConfig загружает конфигурацию из окружения или флагов
func NewConfig() *Config {
	return &Config{
		Port:      configValue("PORT", "port", "8080", "HTTP server port"),
		JWTSecret: configValue("SECRET_KEY", "jwt-secret", "supersecret", "JWT secret key"),

		DB: DBConfig{
			Host:     configValue("PG_HOST", "pg-host", "localhost", "PostgreSQL host"),
			Port:     configValue("PG_PORT", "pg-port", "5432", "PostgreSQL port"),
			User:     configValue("PG_USER", "pg-user", "postgres", "PostgreSQL user"),
			Password: configValue("PG_PASSWORD", "pg-password", "password", "PostgreSQL password"),
			DBName:   configValue("PG_DBNAME", "pg-dbname", "marketgo", "PostgreSQL database name"),
		},
	}
}

// configValue returns the value of a parameter based on the following priority:
// 1. Environment variable.
// 2. Command-line flag.
// 3. Default value.
func configValue(envVar, flagName, defaultValue, description string) string {

	envValue := os.Getenv(envVar)
	if envValue != "" {
		return envValue
	}

	// Create and parse a command-line flag.
	flagValue := flag.String(flagName, defaultValue, description)
	flag.Parse()
	return *flagValue
}
