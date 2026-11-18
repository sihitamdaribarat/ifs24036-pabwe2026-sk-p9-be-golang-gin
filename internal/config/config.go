// Package config memuat konfigurasi aplikasi dari file .env dan environment variable
// (mirip konfigurasi Laravel).
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config menyimpan seluruh konfigurasi runtime aplikasi.
type Config struct {
	AppEnv  string
	AppPort string
	AppKey  string

	AdminUsername string
	AdminPassword string

	DBConnection string
	DBHost       string
	DBPort       string
	DBDatabase   string
	DBUsername   string
	DBPassword   string
	DBSSLMode    string

	SearchEngineEndpoint string
	OpenAIEndpoint       string
	OpenAIKey            string
	OpenAIModel          string
}

// Load membaca file .env (jika ada) lalu menyusun Config dari environment variable.
// Parameter files bersifat opsional; tanpa argumen akan membaca ".env" di direktori kerja.
func Load(files ...string) *Config {
	// File .env bersifat opsional (misal saat CI/CD atau produksi memakai env asli).
	_ = godotenv.Load(files...)

	return &Config{
		AppEnv:  Get("APP_ENV", "development"),
		AppPort: Get("APP_PORT", "8081"),
		AppKey:  Get("APP_KEY", "delcom_blog_default_secret_key"),

		AdminUsername: Get("ADMIN_USERNAME", "admin"),
		AdminPassword: Get("ADMIN_PASSWORD", "admin"),

		DBConnection: Get("DB_CONNECTION", "pgsql"),
		DBHost:       Get("DB_HOST", "127.0.0.1"),
		DBPort:       Get("DB_PORT", "5432"),
		DBDatabase:   Get("DB_DATABASE", "pabwe_p9_blog"),
		DBUsername:   Get("DB_USERNAME", "postgres"),
		DBPassword:   Get("DB_PASSWORD", "postgres"),
		DBSSLMode:    Get("DB_SSLMODE", "disable"),

		SearchEngineEndpoint: Get("SEARCH_ENGINE_ENDPOINT", "https://delcom.org/api/v1/search"),
		OpenAIEndpoint:       Get("OPENAI_ENDPOINT", "https://delcom.org/api/v1"),
		OpenAIKey:            Get("OPENAI_KEY", ""),
		OpenAIModel:          Get("OPENAI_MODEL", ""),
	}
}

// Get mengambil nilai environment variable atau fallback jika kosong.
func Get(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// DSN menyusun connection string PostgreSQL.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Jakarta",
		c.DBHost, c.DBUsername, c.DBPassword, c.DBDatabase, c.DBPort, c.DBSSLMode,
	)
}

// Address mengembalikan alamat listen server Gin (misal ":8081").
func (c *Config) Address() string {
	return ":" + c.AppPort
}

// IsProduction bernilai true jika APP_ENV=production.
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}
