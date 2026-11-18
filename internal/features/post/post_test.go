package post

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"app/internal/features/auth"
	"app/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const testSecret = "post-test-secret"

var errDB = errors.New("db error")

type env struct {
	db     *gorm.DB
	router *gin.Engine
	admin  auth.User
	budi   auth.User
	andi   auth.User
}

func setup(t *testing.T) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&auth.User{}, &Post{}))

	e := &env{db: db}
	e.admin = auth.User{Name: "Admin", Username: "admin", Email: "admin@example.com", Password: "x", Role: auth.RoleAdmin}
	e.budi = auth.User{Name: "Budi", Username: "budi", Email: "budi@example.com", Password: "x", Role: auth.RoleMember}
	e.andi = auth.User{Name: "Andi", Username: "andi", Email: "andi@example.com", Password: "x", Role: auth.RoleMember}
	require.NoError(t, db.Create(&e.admin).Error)
	require.NoError(t, db.Create(&e.budi).Error)
	require.NoError(t, db.Create(&e.andi).Error)

	h := NewHandler(NewService(NewRepository(db)))
	r := gin.New()
	r.GET("/api/public/categories", h.Categories)
	r.GET("/api/public/posts", h.ListPublic)
	r.GET("/api/public/posts/:slug", h.GetPublic)

	authed := r.Group("/api", middleware.Auth(testSecret))
	authed.GET("/posts", h.ListMine)
	authed.POST("/posts", h.Create)
	authed.GET("/posts/stats", h.Stats)
	authed.GET("/posts/:id", h.GetMine)
	authed.PUT("/posts/:id", h.Update)
	authed.DELETE("/posts/:id", h.Delete)
	authed.PATCH("/posts/:id/toggle-publish", h.TogglePublish)
	authed.GET("/admin/users", h.AdminUsers)
	authed.GET("/admin/posts", h.AdminPosts)
	authed.GET("/admin/users/:id/posts", h.AdminUserPosts)

	e.router = r
	return e
}

func token(u auth.User) string {
	t, _ := middleware.GenerateToken(testSecret, u.ID, u.Username, u.Role)
	return t
}

type result struct {
	Code int
	Body map[string]any
}

func (r result) data() map[string]any { return r.Body["data"].(map[string]any) }
func (r result) list() []any          { return r.Body["data"].([]any) }

func (e *env) call(method, path string, user *auth.User, body any) result {
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
	if user != nil {
		req.Header.Set("Authorization", "Bearer "+token(*user))
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)

	var parsed map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &parsed)
	return result{Code: w.Code, Body: parsed}
}

func (e *env) create(t *testing.T, user auth.User, req PostRequest) map[string]any {
	t.Helper()
	res := e.call(http.MethodPost, "/api/posts", &user, req)
	require.Equal(t, http.StatusCreated, res.Code, res.Body)
	return res.data()
}

func TestSlugify(t *testing.T) {
	assert.Equal(t, "belajar-golang-gin-2026", Slugify("  Belajar Golang & Gin: 2026!! "))
	assert.Equal(t, "caf-ber", Slugify("Café Über"))
	assert.Equal(t, "artikel", Slugify("!!! ???"))
	long := Slugify(strings.Repeat("ab ", 150))
	assert.LessOrEqual(t, len(long), 200)
	assert.False(t, strings.HasSuffix(long, "-"))
}

func TestReadingTime(t *testing.T) {
	assert.Equal(t, 1, ReadingTime(""))
	assert.Equal(t, 1, ReadingTime(strings.Repeat("kata ", 200)))
	assert.Equal(t, 2, ReadingTime(strings.Repeat("kata ", 201)))
	assert.Equal(t, 5, ReadingTime(strings.Repeat("kata ", 1000)))
}

func TestNormalizeCategory(t *testing.T) {
	c, err := NormalizeCategory("")
	assert.NoError(t, err)
	assert.Equal(t, DefaultCategory, c)

	c, err = NormalizeCategory(" gaya hidup ")
	assert.NoError(t, err)
	assert.Equal(t, "Gaya Hidup", c)

	_, err = NormalizeCategory("Olahraga")
	assert.ErrorIs(t, err, ErrInvalidCategory)
}

func TestToResponseWithoutAuthor(t *testing.T) {
	res := ToResponse(Post{ID: 1, Title: "T", Content: "a b c"})
	assert.Nil(t, res.Author)
	assert.Equal(t, 1, res.ReadingTime)
	assert.Empty(t, ToResponses(nil))
}

func TestCategoriesEndpoint(t *testing.T) {
	e := setup(t)
	res := e.call(http.MethodGet, "/api/public/categories", nil, nil)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Len(t, res.list(), len(Categories))
}

func TestAuthorCRUDFlow(t *testing.T) {
	e := setup(t)

	draft := e.create(t, e.budi, PostRequest{Title: "Belajar Golang", Category: "teknologi", Excerpt: " Ringkasan ", Content: "isi artikel golang"})
	assert.Equal(t, "belajar-golang", draft["slug"])
	assert.Equal(t, "Teknologi", draft["category"])
	assert.Equal(t, "Ringkasan", draft["excerpt"])
	assert.Equal(t, false, draft["is_published"])
	assert.Nil(t, draft["published_at"])
	assert.Equal(t, float64(1), draft["reading_time"])
	assert.Equal(t, "budi", draft["author"].(map[string]any)["username"])

	dup := e.create(t, e.budi, PostRequest{Title: "Belajar Golang", Content: "lagi"})
	assert.Equal(t, "belajar-golang-2", dup["slug"])
	assert.Equal(t, DefaultCategory, dup["category"])

	pub := e.create(t, e.budi, PostRequest{Title: "Bisnis Digital", Category: "Bisnis", Content: "isi bisnis", IsPublished: true})
	assert.Equal(t, true, pub["is_published"])
	assert.NotNil(t, pub["published_at"])

	e.create(t, e.andi, PostRequest{Title: "Artikel Andi", Content: "milik andi", IsPublished: true})

	// Validasi input.
	assert.Equal(t, http.StatusBadRequest, e.call(http.MethodPost, "/api/posts", &e.budi, "{bad").Code)
	assert.Equal(t, http.StatusBadRequest, e.call(http.MethodPost, "/api/posts", &e.budi, PostRequest{Title: "T", Content: "c", Category: "Olahraga"}).Code)

	// Daftar artikel milik penulis + filter.
	assert.Len(t, e.call(http.MethodGet, "/api/posts", &e.budi, nil).list(), 3)
	assert.Len(t, e.call(http.MethodGet, "/api/posts?q=GOLANG", &e.budi, nil).list(), 2)
	assert.Len(t, e.call(http.MethodGet, "/api/posts?category=Bisnis", &e.budi, nil).list(), 1)

	// Statistik.
	stats := e.call(http.MethodGet, "/api/posts/stats", &e.budi, nil).data()
	assert.Equal(t, float64(3), stats["total"])
	assert.Equal(t, float64(1), stats["published"])
	assert.Equal(t, float64(2), stats["draft"])

	draftID := int(draft["id"].(float64))
	path := fmt.Sprintf("/api/posts/%d", draftID)

	// Detail.
	assert.Equal(t, http.StatusOK, e.call(http.MethodGet, path, &e.budi, nil).Code)
	assert.Equal(t, http.StatusNotFound, e.call(http.MethodGet, path, &e.andi, nil).Code)
	assert.Equal(t, http.StatusNotFound, e.call(http.MethodGet, "/api/posts/9999", &e.budi, nil).Code)
	assert.Equal(t, http.StatusBadRequest, e.call(http.MethodGet, "/api/posts/abc", &e.budi, nil).Code)
	assert.Equal(t, http.StatusBadRequest, e.call(http.MethodGet, "/api/posts/0", &e.budi, nil).Code)

	// Update: judul sama → slug tetap.
	res := e.call(http.MethodPut, path, &e.budi, PostRequest{Title: "Belajar Golang", Category: "Pendidikan", Content: "konten baru"})
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "belajar-golang", res.data()["slug"])
	assert.Equal(t, "Pendidikan", res.data()["category"])

	// Update: judul berubah → slug baru, sekaligus diterbitkan.
	res = e.call(http.MethodPut, path, &e.budi, PostRequest{Title: "Golang Lanjutan", Content: "konten", IsPublished: true})
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "golang-lanjutan", res.data()["slug"])
	assert.Equal(t, true, res.data()["is_published"])

	assert.Equal(t, http.StatusBadRequest, e.call(http.MethodPut, path, &e.budi, "{bad").Code)
	assert.Equal(t, http.StatusBadRequest, e.call(http.MethodPut, "/api/posts/abc", &e.budi, nil).Code)
	assert.Equal(t, http.StatusBadRequest, e.call(http.MethodPut, path, &e.budi, PostRequest{Title: "x", Content: "y", Category: "zzz"}).Code)
	assert.Equal(t, http.StatusNotFound, e.call(http.MethodPut, path, &e.andi, PostRequest{Title: "x", Content: "y"}).Code)

	// Toggle publish: terbit → draf → terbit.
	togglePath := path + "/toggle-publish"
	res = e.call(http.MethodPatch, togglePath, &e.budi, nil)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, false, res.data()["is_published"])
	assert.Nil(t, res.data()["published_at"])
	assert.Equal(t, "Artikel dikembalikan ke draf", res.Body["message"])

	res = e.call(http.MethodPatch, togglePath, &e.budi, nil)
	assert.Equal(t, true, res.data()["is_published"])
	assert.Equal(t, "Artikel berhasil diterbitkan", res.Body["message"])

	assert.Equal(t, http.StatusBadRequest, e.call(http.MethodPatch, "/api/posts/abc/toggle-publish", &e.budi, nil).Code)
	assert.Equal(t, http.StatusNotFound, e.call(http.MethodPatch, togglePath, &e.andi, nil).Code)

	// Delete.
	assert.Equal(t, http.StatusBadRequest, e.call(http.MethodDelete, "/api/posts/abc", &e.budi, nil).Code)
	assert.Equal(t, http.StatusNotFound, e.call(http.MethodDelete, path, &e.andi, nil).Code)
	assert.Equal(t, http.StatusOK, e.call(http.MethodDelete, path, &e.budi, nil).Code)
	assert.Equal(t, http.StatusNotFound, e.call(http.MethodDelete, path, &e.budi, nil).Code)
}

func TestPublicEndpoints(t *testing.T) {
	e := setup(t)
	e.create(t, e.budi, PostRequest{Title: "Draf Rahasia", Content: "draft"})
	e.create(t, e.budi, PostRequest{Title: "AI di Sekolah", Category: "Pendidikan", Excerpt: "kecerdasan buatan", Content: "isi", IsPublished: true})
	e.create(t, e.andi, PostRequest{Title: "Tips Hidup Sehat", Category: "Gaya Hidup", Content: "olahraga", IsPublished: true})

	res := e.call(http.MethodGet, "/api/public/posts", nil, nil)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Len(t, res.list(), 2)

	assert.Len(t, e.call(http.MethodGet, "/api/public/posts?q=kecerdasan", nil, nil).list(), 1)
	assert.Len(t, e.call(http.MethodGet, "/api/public/posts?category=Gaya%20Hidup", nil, nil).list(), 1)
	assert.Len(t, e.call(http.MethodGet, "/api/public/posts?q=tidakada", nil, nil).list(), 0)

	res = e.call(http.MethodGet, "/api/public/posts/ai-di-sekolah", nil, nil)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "AI di Sekolah", res.data()["title"])
	assert.Equal(t, "budi", res.data()["author"].(map[string]any)["username"])

	assert.Equal(t, http.StatusNotFound, e.call(http.MethodGet, "/api/public/posts/draf-rahasia", nil, nil).Code)
	assert.Equal(t, http.StatusNotFound, e.call(http.MethodGet, "/api/public/posts/tidak-ada", nil, nil).Code)
}

func TestAdminEndpoints(t *testing.T) {
	e := setup(t)
	e.create(t, e.budi, PostRequest{Title: "Satu", Content: "a", IsPublished: true})
	e.create(t, e.budi, PostRequest{Title: "Dua", Content: "b"})
	e.create(t, e.andi, PostRequest{Title: "Tiga", Content: "c"})

	users := e.call(http.MethodGet, "/api/admin/users", &e.admin, nil).list()
	require.Len(t, users, 3)
	budi := users[1].(map[string]any)
	assert.Equal(t, "budi", budi["username"])
	assert.Equal(t, float64(2), budi["total_posts"])
	assert.Equal(t, float64(1), budi["published_posts"])
	assert.Equal(t, float64(0), users[0].(map[string]any)["total_posts"])

	assert.Len(t, e.call(http.MethodGet, "/api/admin/posts", &e.admin, nil).list(), 3)
	assert.Len(t, e.call(http.MethodGet, "/api/admin/posts?q=tiga", &e.admin, nil).list(), 1)

	res := e.call(http.MethodGet, fmt.Sprintf("/api/admin/users/%d/posts", e.budi.ID), &e.admin, nil)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Len(t, res.list(), 2)

	assert.Equal(t, http.StatusNotFound, e.call(http.MethodGet, "/api/admin/users/9999/posts", &e.admin, nil).Code)
	assert.Equal(t, http.StatusBadRequest, e.call(http.MethodGet, "/api/admin/users/abc/posts", &e.admin, nil).Code)
}

func TestDatabaseErrors(t *testing.T) {
	e := setup(t)
	sqlDB, _ := e.db.DB()
	require.NoError(t, sqlDB.Close())

	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/public/posts"},
		{http.MethodGet, "/api/public/posts/slug"},
		{http.MethodGet, "/api/posts"},
		{http.MethodGet, "/api/posts/stats"},
		{http.MethodGet, "/api/posts/1"},
		{http.MethodPatch, "/api/posts/1/toggle-publish"},
		{http.MethodDelete, "/api/posts/1"},
		{http.MethodGet, "/api/admin/users"},
		{http.MethodGet, "/api/admin/posts"},
		{http.MethodGet, "/api/admin/users/1/posts"},
	}
	for _, tc := range cases {
		res := e.call(tc.method, tc.path, &e.admin, nil)
		assert.Equal(t, http.StatusInternalServerError, res.Code, tc.path)
		assert.Equal(t, "error", res.Body["status"], tc.path)
	}

	body := PostRequest{Title: "Judul", Content: "isi"}
	assert.Equal(t, http.StatusInternalServerError, e.call(http.MethodPost, "/api/posts", &e.budi, body).Code)
	assert.Equal(t, http.StatusInternalServerError, e.call(http.MethodPut, "/api/posts/1", &e.budi, body).Code)
}

// fakeRepo membungkus repository asli dan menyuntikkan error tertentu.
type fakeRepo struct {
	Repository
	createErr, updateErr, slugErr error
}

func (f *fakeRepo) Create(p *Post) error {
	if f.createErr != nil {
		return f.createErr
	}
	return f.Repository.Create(p)
}

func (f *fakeRepo) Update(p *Post) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	return f.Repository.Update(p)
}

func (f *fakeRepo) SlugExists(slug string, excludeID uint) (bool, error) {
	if f.slugErr != nil {
		return false, f.slugErr
	}
	return f.Repository.SlugExists(slug, excludeID)
}

func TestServiceErrors(t *testing.T) {
	e := setup(t)
	repo := &fakeRepo{Repository: NewRepository(e.db)}
	svc := NewService(repo)

	repo.createErr = errDB
	_, err := svc.Create(e.budi.ID, PostRequest{Title: "A", Content: "a"})
	assert.ErrorIs(t, err, errDB)
	repo.createErr = nil

	created, err := svc.Create(e.budi.ID, PostRequest{Title: "A", Content: "a"})
	require.NoError(t, err)

	repo.slugErr = errDB
	_, err = svc.Update(e.budi.ID, created.ID, PostRequest{Title: "B", Content: "b"})
	assert.ErrorIs(t, err, errDB)
	repo.slugErr = nil

	repo.updateErr = errDB
	_, err = svc.Update(e.budi.ID, created.ID, PostRequest{Title: "A", Content: "b"})
	assert.ErrorIs(t, err, errDB)

	_, err = svc.TogglePublish(e.budi.ID, created.ID)
	assert.ErrorIs(t, err, errDB)
}
