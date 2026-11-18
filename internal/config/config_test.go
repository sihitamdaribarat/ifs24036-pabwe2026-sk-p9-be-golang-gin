package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadDefaults(t *testing.T) {
	keys := []string{"APP_ENV", "APP_PORT", "OPENAI_KEY", "DB_CONNECTION"}
	for _, k := range keys {
		t.Setenv(k, "")
	}

	cfg := Load(filepath.Join(t.TempDir(), "missing.env"))

	assert.Equal(t, "development", cfg.AppEnv)
	assert.Equal(t, "8081", cfg.AppPort)
	assert.Equal(t, "pgsql", cfg.DBConnection)
	assert.Equal(t, "", cfg.OpenAIKey)
	assert.Equal(t, ":8081", cfg.Address())
	assert.False(t, cfg.IsProduction())
}

func TestLoadFromEnvFile(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	content := "APP_ENV=production\nAPP_PORT=9999\nDB_DATABASE=testdb\nSEARCH_ENGINE_ENDPOINT=http://search.local\n"
	assert.NoError(t, os.WriteFile(envFile, []byte(content), 0o600))

	for _, k := range []string{"APP_ENV", "APP_PORT", "DB_DATABASE", "SEARCH_ENGINE_ENDPOINT"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}

	cfg := Load(envFile)

	assert.Equal(t, "production", cfg.AppEnv)
	assert.Equal(t, "9999", cfg.AppPort)
	assert.Equal(t, "testdb", cfg.DBDatabase)
	assert.Equal(t, "http://search.local", cfg.SearchEngineEndpoint)
	assert.True(t, cfg.IsProduction())
}

func TestGet(t *testing.T) {
	t.Setenv("CONFIG_TEST_KEY", "value")
	assert.Equal(t, "value", Get("CONFIG_TEST_KEY", "fallback"))
	assert.Equal(t, "fallback", Get("CONFIG_TEST_MISSING_KEY", "fallback"))
}

func TestDSN(t *testing.T) {
	cfg := &Config{
		DBHost: "localhost", DBUsername: "user", DBPassword: "pass",
		DBDatabase: "db", DBPort: "5432", DBSSLMode: "disable",
	}
	assert.Equal(t,
		"host=localhost user=user password=pass dbname=db port=5432 sslmode=disable TimeZone=Asia/Jakarta",
		cfg.DSN(),
	)
}
