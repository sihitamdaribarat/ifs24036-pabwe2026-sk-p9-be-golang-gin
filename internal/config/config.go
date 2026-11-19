// Package config memuat konfigurasi aplikasi dari file .env dan environment variable
// (mirip konfigurasi Laravel).
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config menyimpan seluruh konfigurasi runtime aplikasi.
type Config struct {
	AppEnv  string
	AppPort string
	AppKey  string

	AdminUsername string
	AdminPassword string
	// AllowAdminRegistration mengizinkan registrasi dengan "role": "admin" (misal untuk
	// penguji otomatis). Bawaan false karena membuka akses admin ke siapa pun.
	AllowAdminRegistration bool

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
	// File .env bersifat opsional (misal saat CI/CD tanpa .env). Jika ada, nilainya
	// menimpa environment variable lama: platform deployment membuat ulang .env
	// (PORT & DB_PASSWORD baru) setiap deploy, sehingga .env adalah sumber kebenaran.
	_ = godotenv.Overload(files...)

	return &Config{
		AppEnv:  Get("APP_ENV", "development"),
		AppPort: Get("APP_PORT", "8081"),
		AppKey:  Get("APP_KEY", "delcom_blog_default_secret_key"),

		AdminUsername: Get("ADMIN_USERNAME", "admin"),
		AdminPassword: Get("ADMIN_PASSWORD", "admin"),

		AllowAdminRegistration: strings.EqualFold(Get("ALLOW_ADMIN_REGISTRATION", "false"), "true"),

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

// DSN menyusun connection string PostgreSQL. Tidak memakai parameter TimeZone karena
// PgBouncer menolak startup parameter yang tidak dikenal; password diberi tanda kutip
// agar karakter khusus tetap aman.
func (c *Config) DSN() string {
	password := strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(c.DBPassword)
	return fmt.Sprintf(
		"host=%s user=%s password='%s' dbname=%s port=%s sslmode=%s",
		c.DBHost, c.DBUsername, password, c.DBDatabase, c.DBPort, c.DBSSLMode,
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
