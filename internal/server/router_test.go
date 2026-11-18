package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"app/internal/config"
	"app/internal/database"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Percepat retry koneksi database saat pengujian.
	dbConnectDelay = time.Millisecond
}

func testConfig() *config.Config {
	return &config.Config{
		AppEnv:               "testing",
		AppPort:              "8081",
		AppKey:               "router-test-secret",
		AdminUsername:        "admin",
		AdminPassword:        "admin",
		SearchEngineEndpoint: "http://127.0.0.1:1/search",
		OpenAIEndpoint:       "http://127.0.0.1:1/v1",
	}
}

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := database.ConnectTest()
	require.NoError(t, err)
	require.NoError(t, database.Seed(db, "admin", "admin"))
	return NewRouter(db, testConfig())
}

func call(r http.Handler, method, path, token string, body any) (int, map[string]any) {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var parsed map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &parsed)
	return w.Code, parsed
}

func loginToken(t *testing.T, r http.Handler, username, password string) string {
	t.Helper()
	code, body := call(r, http.MethodPost, "/api/auth/login", "", map[string]string{"username": username, "password": password})
	require.Equal(t, http.StatusOK, code, body)
	return body["data"].(map[string]any)["token"].(string)
}

func TestNoRoute(t *testing.T) {
	r := newTestRouter(t)
	code, body := call(r, http.MethodGet, "/api/unknown", "", nil)
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, "fail", body["status"])
	assert.Equal(t, "Endpoint not found", body["message"])
}

func TestHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.ConnectTest()
	require.NoError(t, err)
	require.NoError(t, database.Seed(db, "admin", "admin"))

	cfg := testConfig()
	r := NewRouter(db, cfg)
	code, body := call(r, http.MethodGet, "/api/health", "", nil)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(1), body["data"].(map[string]any)["users"])

	// Tabel hilang → 503 dengan detail error (mode non-production).
	require.NoError(t, db.Migrator().DropTable("posts", "users"))
	code, body = call(r, http.MethodGet, "/api/health", "", nil)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Contains(t, body["message"], "Database tidak dapat diakses: ")

	// Mode production → detail error disembunyikan.
	cfg.AppEnv = "production"
	defer gin.SetMode(gin.TestMode)
	r = NewRouter(db, cfg)
	code, body = call(r, http.MethodGet, "/api/health", "", nil)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "Database tidak dapat diakses", body["message"])
}

func TestRootRedirectsToSwagger(t *testing.T) {
	r := newTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/swagger/index.html", w.Header().Get("Location"))
}

func TestSwaggerAndAPIJSON(t *testing.T) {
	r := newTestRouter(t)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "swagger")

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api.json", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	assert.True(t, json.Valid(w.Body.Bytes()))
	assert.Contains(t, w.Body.String(), "/api/posts/ai-generate")
}

func TestCORSPreflight(t *testing.T) {
	r := newTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodOptions, "/api/posts", nil))
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestPublicRoutes(t *testing.T) {
	r := newTestRouter(t)

	code, body := call(r, http.MethodGet, "/api/public/categories", "", nil)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, body["data"], 6)

	code, body = call(r, http.MethodGet, "/api/public/posts", "", nil)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, body["data"], 1)

	code, _ = call(r, http.MethodGet, "/api/public/posts/selamat-datang-di-delcom-blog", "", nil)
	assert.Equal(t, http.StatusOK, code)
}

func TestMultiTierAccess(t *testing.T) {
	r := newTestRouter(t)

	code, _ := call(r, http.MethodPost, "/api/auth/register", "", map[string]string{
		"name": "Budi", "username": "budi", "email": "budi@example.com", "password": "secret123",
	})
	require.Equal(t, http.StatusCreated, code)

	member := loginToken(t, r, "budi", "secret123")
	admin := loginToken(t, r, "admin", "admin")

	// Tanpa token.
	code, _ = call(r, http.MethodGet, "/api/posts", "", nil)
	assert.Equal(t, http.StatusUnauthorized, code)

	// Penulis.
	code, _ = call(r, http.MethodPost, "/api/posts", member, map[string]any{"title": "Halo", "content": "isi", "is_published": true})
	assert.Equal(t, http.StatusCreated, code)
	code, body := call(r, http.MethodGet, "/api/posts/stats", member, nil)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(1), body["data"].(map[string]any)["published"])
	code, _ = call(r, http.MethodGet, "/api/auth/profile", member, nil)
	assert.Equal(t, http.StatusOK, code)

	// Penulis tidak boleh mengakses admin.
	code, _ = call(r, http.MethodGet, "/api/admin/users", member, nil)
	assert.Equal(t, http.StatusForbidden, code)

	// Admin.
	code, body = call(r, http.MethodGet, "/api/admin/users", admin, nil)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, body["data"], 2)
	code, body = call(r, http.MethodGet, "/api/admin/posts", admin, nil)
	assert.Equal(t, http.StatusOK, code)
	assert.Len(t, body["data"], 2)

	// AI generator: OPENAI_KEY belum diatur → 503.
	code, body = call(r, http.MethodPost, "/api/posts/ai-generate", member, map[string]any{"topic": "AI", "include_web_search": true})
	assert.Equal(t, http.StatusServiceUnavailable, code, body)

	code, _ = call(r, http.MethodPost, "/api/auth/logout", member, nil)
	assert.Equal(t, http.StatusOK, code)
	code, _ = call(r, http.MethodGet, "/api/posts", member, nil)
	assert.Equal(t, http.StatusUnauthorized, code)
}

func TestProductionMode(t *testing.T) {
	defer gin.SetMode(gin.TestMode)
	db, err := database.ConnectTest()
	require.NoError(t, err)

	cfg := testConfig()
	cfg.AppEnv = "production"
	NewRouter(db, cfg)
	assert.Equal(t, gin.ReleaseMode, gin.Mode())
}

func tempDBPath(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(os.TempDir(), fmt.Sprintf("delcom-bootstrap-%d.db", time.Now().UnixNano()))
	if content != "" {
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	// Pada Windows berkas SQLite yang masih terbuka tidak dapat dihapus; abaikan error.
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func TestBootstrap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testConfig()
	cfg.DBConnection = "sqlite"
	cfg.DBDatabase = tempDBPath(t, "")

	r, err := Bootstrap(cfg)
	require.NoError(t, err)
	code, _ := call(r, http.MethodGet, "/api/public/posts", "", nil)
	assert.Equal(t, http.StatusOK, code)
}

func TestBootstrapErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := testConfig()
	cfg.DBConnection = "unsupported"
	_, err := Bootstrap(cfg)
	assert.Error(t, err)

	// Berkas bukan database SQLite → migrasi gagal.
	cfg = testConfig()
	cfg.DBConnection = "sqlite"
	cfg.DBDatabase = tempDBPath(t, strings.Repeat("bukan database sqlite ", 200))
	_, err = Bootstrap(cfg)
	assert.Error(t, err)

	// Password admin melebihi 72 byte → seeding gagal.
	cfg = testConfig()
	cfg.DBConnection = "sqlite"
	cfg.DBDatabase = tempDBPath(t, "")
	cfg.AdminPassword = strings.Repeat("x", 100)
	_, err = Bootstrap(cfg)
	assert.Error(t, err)
}
