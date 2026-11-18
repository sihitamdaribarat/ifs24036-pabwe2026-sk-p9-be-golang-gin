// Package middleware berisi middleware Gin: CORS, autentikasi JWT, pemeriksaan role
// dan rate limiter.
package middleware

import (
	"crypto/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"app/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	// ContextUserID adalah key gin.Context untuk ID user yang sedang login.
	ContextUserID = "user_id"
	// ContextUsername adalah key gin.Context untuk username yang sedang login.
	ContextUsername = "username"
	// ContextRole adalah key gin.Context untuk role user yang sedang login.
	ContextRole = "role"
	// ContextToken adalah key gin.Context untuk token JWT mentah.
	ContextToken = "token"

	// TokenTTL adalah masa berlaku token JWT.
	TokenTTL = 24 * time.Hour
)

// Claims adalah payload token JWT Delcom Blog.
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// revokedTokens menyimpan token yang sudah logout hingga masa berlakunya habis.
var revokedTokens sync.Map

// GenerateToken membuat token JWT HS256 untuk user.
func GenerateToken(secret string, userID uint, username, role string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			// ID unik agar setiap sesi login memiliki token berbeda (logout tidak saling memengaruhi).
			ID:        rand.Text(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// RevokeToken menandai token sebagai tidak berlaku (logout).
func RevokeToken(token string) {
	revokedTokens.Store(token, time.Now().Add(TokenTTL))
}

// IsTokenRevoked memeriksa apakah token sudah di-logout. Entri kedaluwarsa dibersihkan.
func IsTokenRevoked(token string) bool {
	value, ok := revokedTokens.Load(token)
	if !ok {
		return false
	}
	if time.Now().After(value.(time.Time)) {
		revokedTokens.Delete(token)
		return false
	}
	return true
}

// CORS mengizinkan permintaan lintas origin dari frontend SPA.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		h.Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Auth memvalidasi header "Authorization: Bearer <token>".
func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tokenString, found := strings.CutPrefix(header, "Bearer ")
		if !found || strings.TrimSpace(tokenString) == "" {
			response.Fail(c, http.StatusUnauthorized, "Token autentikasi tidak ditemukan")
			return
		}

		claims := &Claims{}
		_, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (any, error) {
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || IsTokenRevoked(tokenString) {
			response.Fail(c, http.StatusUnauthorized, "Token tidak valid atau sudah kedaluwarsa")
			return
		}

		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextUsername, claims.Username)
		c.Set(ContextRole, claims.Role)
		c.Set(ContextToken, tokenString)
		c.Next()
	}
}

// RequireRole hanya meneruskan request jika role user termasuk dalam daftar roles.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString(ContextRole)
		for _, allowed := range roles {
			if role == allowed {
				c.Next()
				return
			}
		}
		response.Fail(c, http.StatusForbidden, "Anda tidak memiliki akses ke sumber daya ini")
	}
}

// UserID mengambil ID user yang sedang login dari context.
func UserID(c *gin.Context) uint {
	return c.GetUint(ContextUserID)
}

// RateLimiter membatasi jumlah request per IP dalam jendela waktu tertentu (fixed window).
func RateLimiter(limit int, window time.Duration) gin.HandlerFunc {
	type visitor struct {
		count int
		reset time.Time
	}
	var mu sync.Mutex
	visitors := map[string]*visitor{}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		v, ok := visitors[ip]
		if !ok || now.After(v.reset) {
			v = &visitor{reset: now.Add(window)}
			visitors[ip] = v
		}
		v.count++
		exceeded := v.count > limit
		mu.Unlock()

		if exceeded {
			response.Fail(c, http.StatusTooManyRequests, "Terlalu banyak permintaan, silakan coba lagi nanti")
			return
		}
		c.Next()
	}
}
