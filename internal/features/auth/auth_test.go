package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"app/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const testSecret = "auth-test-secret"

var errDB = errors.New("db error")

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&User{}))
	return db
}

func setupRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(NewRepository(db), testSecret))
	r := gin.New()
	r.POST("/api/auth/register", h.Register)
	r.POST("/api/auth/login", h.Login)
	authed := r.Group("/api/auth", middleware.Auth(testSecret))
	authed.POST("/logout", h.Logout)
	authed.GET("/profile", h.GetProfile)
	authed.PUT("/profile", h.UpdateProfile)
	return r
}

type result struct {
	Code int
	Body map[string]any
}

func call(r http.Handler, method, path, token string, body any) result {
	var buf bytes.Buffer
	switch v := body.(type) {
	case nil:
	case string:
		buf.WriteString(v)
	default:
		_ = json.NewEncoder(&buf).Encode(v)
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
	return result{Code: w.Code, Body: parsed}
}

func register(r http.Handler, username, email string) result {
	return call(r, http.MethodPost, "/api/auth/register", "", RegisterRequest{
		Name: "User " + username, Username: username, Email: email, Password: "secret123",
	})
}

func login(t *testing.T, r http.Handler, username, password string) string {
	t.Helper()
	res := call(r, http.MethodPost, "/api/auth/login", "", LoginRequest{Username: username, Password: password})
	require.Equal(t, http.StatusOK, res.Code)
	return res.Body["data"].(map[string]any)["token"].(string)
}

func TestRegister(t *testing.T) {
	r := setupRouter(setupDB(t))

	// Registrasi langsung menerbitkan token JWT (auto login).
	res := register(r, "Budi", "Budi@Example.com")
	assert.Equal(t, http.StatusCreated, res.Code)
	data := res.Body["data"].(map[string]any)
	assert.Len(t, strings.Split(data["token"].(string), "."), 3)
	user := data["user"].(map[string]any)
	assert.Equal(t, "budi", user["username"])
	assert.Equal(t, "budi@example.com", user["email"])
	assert.Equal(t, RoleMember, user["role"])
	_, hasPassword := user["password"]
	assert.False(t, hasPassword)

	// Duplikat & data tidak valid → 400.
	assert.Equal(t, http.StatusBadRequest, register(r, "budi", "other@example.com").Code)
	assert.Equal(t, http.StatusBadRequest, register(r, "andi", "budi@example.com").Code)
	assert.Equal(t, http.StatusBadRequest, call(r, http.MethodPost, "/api/auth/register", "", "{bad").Code)
	assert.Equal(t, http.StatusBadRequest, register(r, "x", "invalid-email").Code)
	assert.Equal(t, http.StatusBadRequest, register(r, "budi santoso", "bs@example.com").Code)
	res = call(r, http.MethodPost, "/api/auth/register", "", map[string]string{"username": "tanpaemail", "password": "secret123"})
	assert.Equal(t, http.StatusBadRequest, res.Code)

	// Username dengan _ . - diizinkan; name opsional (bawaan = username).
	res = call(r, http.MethodPost, "/api/auth/register", "", map[string]string{"username": "Test_User-1.a", "email": "t@example.com", "password": "secret123"})
	assert.Equal(t, http.StatusCreated, res.Code)
	user = res.Body["data"].(map[string]any)["user"].(map[string]any)
	assert.Equal(t, "test_user-1.a", user["username"])
	assert.Equal(t, "test_user-1.a", user["name"])
}

func TestRegisterAdminRole(t *testing.T) {
	db := setupDB(t)
	svc := NewService(NewRepository(db), testSecret)
	req := func(username string) RegisterRequest {
		return RegisterRequest{Username: username, Email: username + "@example.com", Password: "secret123", Role: " Admin "}
	}

	// Bawaan: role admin diabaikan → member.
	res, err := svc.Register(req("biasa"))
	require.NoError(t, err)
	assert.Equal(t, RoleMember, res.User.Role)

	// ALLOW_ADMIN_REGISTRATION=true → admin.
	svc.AllowAdminRegistration = true
	res, err = svc.Register(req("bos"))
	require.NoError(t, err)
	assert.Equal(t, RoleAdmin, res.User.Role)
}

func TestLogin(t *testing.T) {
	r := setupRouter(setupDB(t))
	register(r, "budi", "budi@example.com")

	assert.NotEmpty(t, login(t, r, "budi", "secret123"))
	assert.NotEmpty(t, login(t, r, "BUDI@example.com", "secret123"))

	res := call(r, http.MethodPost, "/api/auth/login", "", LoginRequest{Username: "budi", Password: "wrong"})
	assert.Equal(t, http.StatusUnauthorized, res.Code)
	assert.Equal(t, "fail", res.Body["status"])

	res = call(r, http.MethodPost, "/api/auth/login", "", LoginRequest{Username: "nobody", Password: "x"})
	assert.Equal(t, http.StatusUnauthorized, res.Code)

	res = call(r, http.MethodPost, "/api/auth/login", "", map[string]string{"username": "budi"})
	assert.Equal(t, http.StatusBadRequest, res.Code)

	res = call(r, http.MethodPost, "/api/auth/login", "", map[string]string{"password": "secret123"})
	assert.Equal(t, http.StatusBadRequest, res.Code)

	// Login juga bisa lewat field email.
	res = call(r, http.MethodPost, "/api/auth/login", "", map[string]string{"email": "budi@example.com", "password": "secret123"})
	assert.Equal(t, http.StatusOK, res.Code)
}

func TestProfileAndLogout(t *testing.T) {
	db := setupDB(t)
	r := setupRouter(db)
	register(r, "budi", "budi@example.com")
	register(r, "andi", "andi@example.com")
	token := login(t, r, "budi", "secret123")

	res := call(r, http.MethodGet, "/api/auth/profile", token, nil)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "budi", res.Body["data"].(map[string]any)["username"])

	bio := "Penulis"
	res = call(r, http.MethodPut, "/api/auth/profile", token, UpdateProfileRequest{
		Name: "Budi Baru", Email: "budi.baru@example.com", Bio: &bio, Password: "newpass123",
	})
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "Budi Baru", res.Body["data"].(map[string]any)["name"])
	assert.NotEmpty(t, login(t, r, "budi", "newpass123"))

	// Update parsial: hanya bio yang dikirim, field lain tetap.
	res = call(r, http.MethodPut, "/api/auth/profile", token, map[string]string{"bio": "Bio baru"})
	assert.Equal(t, http.StatusOK, res.Code)
	data := res.Body["data"].(map[string]any)
	assert.Equal(t, "Bio baru", data["bio"])
	assert.Equal(t, "Budi Baru", data["name"])
	assert.Equal(t, "budi.baru@example.com", data["email"])

	// Update tanpa password tetap mempertahankan password lama.
	res = call(r, http.MethodPut, "/api/auth/profile", token, UpdateProfileRequest{Name: "Budi", Email: "budi.baru@example.com"})
	assert.Equal(t, http.StatusOK, res.Code)
	assert.NotEmpty(t, login(t, r, "budi", "newpass123"))

	res = call(r, http.MethodPut, "/api/auth/profile", token, UpdateProfileRequest{Name: "Budi", Email: "andi@example.com"})
	assert.Equal(t, http.StatusBadRequest, res.Code)

	res = call(r, http.MethodPut, "/api/auth/profile", token, "{bad")
	assert.Equal(t, http.StatusBadRequest, res.Code)

	res = call(r, http.MethodPost, "/api/auth/logout", token, nil)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, http.StatusUnauthorized, call(r, http.MethodGet, "/api/auth/profile", token, nil).Code)
}

func TestProfileUserDeleted(t *testing.T) {
	db := setupDB(t)
	r := setupRouter(db)
	register(r, "budi", "budi@example.com")
	token := login(t, r, "budi", "secret123")

	require.NoError(t, db.Exec("DELETE FROM users").Error)
	assert.Equal(t, http.StatusNotFound, call(r, http.MethodGet, "/api/auth/profile", token, nil).Code)
	res := call(r, http.MethodPut, "/api/auth/profile", token, UpdateProfileRequest{Name: "B", Email: "b@example.com"})
	assert.Equal(t, http.StatusNotFound, res.Code)
}

func TestDatabaseErrors(t *testing.T) {
	db := setupDB(t)
	r := setupRouter(db)
	register(r, "budi", "budi@example.com")
	token := login(t, r, "budi", "secret123")

	sqlDB, _ := db.DB()
	require.NoError(t, sqlDB.Close())

	res := register(r, "andi", "andi@example.com")
	assert.Equal(t, http.StatusInternalServerError, res.Code)
	assert.Equal(t, "error", res.Body["status"])

	res = call(r, http.MethodPost, "/api/auth/login", "", LoginRequest{Username: "budi", Password: "secret123"})
	assert.Equal(t, http.StatusInternalServerError, res.Code)

	assert.Equal(t, http.StatusInternalServerError, call(r, http.MethodGet, "/api/auth/profile", token, nil).Code)
}

// fakeRepo membungkus repository asli dan menyuntikkan error tertentu.
type fakeRepo struct {
	Repository
	createErr, updateErr, countUserErr, countEmailErr error
}

func (f *fakeRepo) Create(u *User) error {
	if f.createErr != nil {
		return f.createErr
	}
	return f.Repository.Create(u)
}

func (f *fakeRepo) Update(u *User) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	return f.Repository.Update(u)
}

func (f *fakeRepo) CountByUsername(username string) (int64, error) {
	if f.countUserErr != nil {
		return 0, f.countUserErr
	}
	return f.Repository.CountByUsername(username)
}

func (f *fakeRepo) CountByEmail(email string, excludeID uint) (int64, error) {
	if f.countEmailErr != nil {
		return 0, f.countEmailErr
	}
	return f.Repository.CountByEmail(email, excludeID)
}

func TestServiceErrors(t *testing.T) {
	newReq := func(username string) RegisterRequest {
		return RegisterRequest{Name: "N", Username: username, Email: username + "@example.com", Password: "secret123"}
	}

	repo := &fakeRepo{Repository: NewRepository(setupDB(t))}
	svc := NewService(repo, testSecret)

	repo.countUserErr = errDB
	_, err := svc.Register(newReq("a"))
	assert.ErrorIs(t, err, errDB)
	repo.countUserErr = nil

	repo.countEmailErr = errDB
	_, err = svc.Register(newReq("b"))
	assert.ErrorIs(t, err, errDB)
	repo.countEmailErr = nil

	long := newReq("c")
	long.Password = strings.Repeat("x", 100)
	_, err = svc.Register(long)
	assert.Error(t, err)

	repo.createErr = errDB
	_, err = svc.Register(newReq("d"))
	assert.ErrorIs(t, err, errDB)
	repo.createErr = nil

	registered, err := svc.Register(newReq("e"))
	require.NoError(t, err)
	user := registered.User

	_, err = svc.UpdateProfile(user.ID, UpdateProfileRequest{Name: "E", Email: "e@example.com", Password: strings.Repeat("x", 100)})
	assert.Error(t, err)

	repo.updateErr = errDB
	_, err = svc.UpdateProfile(user.ID, UpdateProfileRequest{Name: "E", Email: "e@example.com"})
	assert.ErrorIs(t, err, errDB)
}

func TestStatusFor(t *testing.T) {
	assert.Equal(t, http.StatusBadRequest, statusFor(ErrInvalidUsername))
	assert.Equal(t, http.StatusBadRequest, statusFor(ErrUsernameTaken))
	assert.Equal(t, http.StatusBadRequest, statusFor(ErrEmailTaken))
	assert.Equal(t, http.StatusUnauthorized, statusFor(ErrInvalidCredentials))
	assert.Equal(t, http.StatusNotFound, statusFor(ErrUserNotFound))
	assert.Equal(t, http.StatusInternalServerError, statusFor(errDB))
}
