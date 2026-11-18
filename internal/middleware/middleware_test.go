package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

const secret = "test-secret"

func newEngine(handlers ...gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handlers = append(handlers, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id":  UserID(c),
			"username": c.GetString(ContextUsername),
			"role":     c.GetString(ContextRole),
		})
	})
	r.Any("/test", handlers...)
	return r
}

func doRequest(r http.Handler, method, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/test", nil)
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCORS(t *testing.T) {
	r := newEngine(CORS())

	w := doRequest(r, http.MethodOptions, "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))

	w = doRequest(r, http.MethodGet, "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "PATCH")
}

func TestAuthMissingToken(t *testing.T) {
	r := newEngine(Auth(secret))
	assert.Equal(t, http.StatusUnauthorized, doRequest(r, http.MethodGet, "").Code)
	assert.Equal(t, http.StatusUnauthorized, doRequest(r, http.MethodGet, "Basic abc").Code)
	assert.Equal(t, http.StatusUnauthorized, doRequest(r, http.MethodGet, "Bearer   ").Code)
}

func TestAuthInvalidToken(t *testing.T) {
	r := newEngine(Auth(secret))
	assert.Equal(t, http.StatusUnauthorized, doRequest(r, http.MethodGet, "Bearer invalid.token.here").Code)

	other, _ := GenerateToken("other-secret", 1, "u", "member")
	assert.Equal(t, http.StatusUnauthorized, doRequest(r, http.MethodGet, "Bearer "+other).Code)

	// Algoritma "none" harus ditolak.
	none := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{UserID: 1})
	noneToken, _ := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	assert.Equal(t, http.StatusUnauthorized, doRequest(r, http.MethodGet, "Bearer "+noneToken).Code)
}

func TestAuthValidToken(t *testing.T) {
	r := newEngine(Auth(secret))
	token, err := GenerateToken(secret, 7, "budi", "member")
	assert.NoError(t, err)

	w := doRequest(r, http.MethodGet, "Bearer "+token)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"user_id":7`)
	assert.Contains(t, w.Body.String(), `"username":"budi"`)
}

func TestRevokeToken(t *testing.T) {
	r := newEngine(Auth(secret))
	token, _ := GenerateToken(secret, 8, "ani", "member")

	assert.False(t, IsTokenRevoked(token))
	RevokeToken(token)
	assert.True(t, IsTokenRevoked(token))
	assert.Equal(t, http.StatusUnauthorized, doRequest(r, http.MethodGet, "Bearer "+token).Code)

	// Entri yang sudah kedaluwarsa dibersihkan dan dianggap tidak dicabut.
	revokedTokens.Store("expired-token", time.Now().Add(-time.Minute))
	assert.False(t, IsTokenRevoked("expired-token"))
	_, exists := revokedTokens.Load("expired-token")
	assert.False(t, exists)
}

func TestRequireRole(t *testing.T) {
	setRole := func(role string) gin.HandlerFunc {
		return func(c *gin.Context) { c.Set(ContextRole, role) }
	}

	r := newEngine(setRole("admin"), RequireRole("admin"))
	assert.Equal(t, http.StatusOK, doRequest(r, http.MethodGet, "").Code)

	r = newEngine(setRole("member"), RequireRole("admin"))
	assert.Equal(t, http.StatusForbidden, doRequest(r, http.MethodGet, "").Code)

	r = newEngine(setRole("member"), RequireRole("admin", "member"))
	assert.Equal(t, http.StatusOK, doRequest(r, http.MethodGet, "").Code)
}

func TestRateLimiter(t *testing.T) {
	r := newEngine(RateLimiter(2, 50*time.Millisecond))

	assert.Equal(t, http.StatusOK, doRequest(r, http.MethodGet, "").Code)
	assert.Equal(t, http.StatusOK, doRequest(r, http.MethodGet, "").Code)
	assert.Equal(t, http.StatusTooManyRequests, doRequest(r, http.MethodGet, "").Code)

	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, http.StatusOK, doRequest(r, http.MethodGet, "").Code)
}
