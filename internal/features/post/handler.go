package post

import (
	"errors"
	"net/http"
	"strconv"

	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler adalah HTTP handler endpoint /api/public/*, /api/posts/* dan /api/admin/*.
type Handler struct {
	service *Service
}

// NewHandler membuat Handler artikel.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func respondError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrPostNotFound), errors.Is(err, ErrUserNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrInvalidCategory):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.InternalError(c, err)
	}
}

// paramID membaca parameter :id sebagai uint; mengirim 400 jika tidak valid.
func paramID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "ID tidak valid")
		return 0, false
	}
	return uint(id), true
}

func bindFilter(c *gin.Context) Filter {
	return Filter{Q: c.Query("q"), Category: c.Query("category")}
}

func bindPost(c *gin.Context) (PostRequest, bool) {
	var req PostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data artikel tidak valid: judul dan konten wajib diisi")
		return req, false
	}
	return req, true
}

// Categories godoc
// @Summary      Daftar kategori artikel
// @Tags         Public
// @Produce      json
// @Success      200  {object}  response.Body{data=[]string}
// @Router       /api/public/categories [get]
func (h *Handler) Categories(c *gin.Context) {
	response.Success(c, http.StatusOK, "Daftar kategori berhasil diambil", Categories)
}

// ListPublic godoc
// @Summary      Daftar artikel terbit
// @Tags         Public
// @Produce      json
// @Param        q         query     string  false  "Kata kunci pencarian"
// @Param        category  query     string  false  "Filter kategori"
// @Success      200       {object}  response.Body{data=[]PostResponse}
// @Router       /api/public/posts [get]
func (h *Handler) ListPublic(c *gin.Context) {
	posts, err := h.service.ListPublic(bindFilter(c))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Daftar artikel berhasil diambil", posts)
}

// GetPublic godoc
// @Summary      Detail artikel terbit berdasarkan slug
// @Tags         Public
// @Produce      json
// @Param        slug  path      string  true  "Slug artikel"
// @Success      200   {object}  response.Body{data=PostResponse}
// @Failure      404   {object}  response.Body
// @Router       /api/public/posts/{slug} [get]
func (h *Handler) GetPublic(c *gin.Context) {
	p, err := h.service.GetPublicBySlug(c.Param("slug"))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Detail artikel berhasil diambil", p)
}

// ListMine godoc
// @Summary      Daftar artikel milik penulis yang sedang login
// @Tags         Posts
// @Produce      json
// @Security     BearerAuth
// @Param        q         query     string  false  "Kata kunci pencarian"
// @Param        category  query     string  false  "Filter kategori"
// @Success      200       {object}  response.Body{data=[]PostResponse}
// @Router       /api/posts [get]
func (h *Handler) ListMine(c *gin.Context) {
	posts, err := h.service.ListByAuthor(middleware.UserID(c), bindFilter(c))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Daftar artikel berhasil diambil", posts)
}

// Create godoc
// @Summary      Buat artikel baru
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      PostRequest  true  "Data artikel"
// @Success      201   {object}  response.Body{data=PostResponse}
// @Failure      400   {object}  response.Body
// @Router       /api/posts [post]
func (h *Handler) Create(c *gin.Context) {
	req, ok := bindPost(c)
	if !ok {
		return
	}
	p, err := h.service.Create(middleware.UserID(c), req)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, "Artikel berhasil dibuat", p)
}

// Stats godoc
// @Summary      Ringkasan 3 kartu statistik penulis
// @Tags         Posts
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Body{data=Stats}
// @Router       /api/posts/stats [get]
func (h *Handler) Stats(c *gin.Context) {
	stats, err := h.service.Stats(middleware.UserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Statistik artikel berhasil diambil", stats)
}

// GetMine godoc
// @Summary      Detail artikel milik penulis
// @Tags         Posts
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "ID artikel"
// @Success      200  {object}  response.Body{data=PostResponse}
// @Failure      404  {object}  response.Body
// @Router       /api/posts/{id} [get]
func (h *Handler) GetMine(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	p, err := h.service.GetByAuthor(middleware.UserID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Detail artikel berhasil diambil", p)
}

// Update godoc
// @Summary      Perbarui artikel milik penulis
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      int          true  "ID artikel"
// @Param        body  body      PostRequest  true  "Data artikel"
// @Success      200   {object}  response.Body{data=PostResponse}
// @Failure      400   {object}  response.Body
// @Failure      404   {object}  response.Body
// @Router       /api/posts/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	req, ok := bindPost(c)
	if !ok {
		return
	}
	p, err := h.service.Update(middleware.UserID(c), id, req)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diperbarui", p)
}

// TogglePublish godoc
// @Summary      Ubah status terbit/draf artikel
// @Tags         Posts
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "ID artikel"
// @Success      200  {object}  response.Body{data=PostResponse}
// @Failure      404  {object}  response.Body
// @Router       /api/posts/{id}/toggle-publish [patch]
func (h *Handler) TogglePublish(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	p, err := h.service.TogglePublish(middleware.UserID(c), id)
	if err != nil {
		respondError(c, err)
		return
	}
	message := "Artikel dikembalikan ke draf"
	if p.IsPublished {
		message = "Artikel berhasil diterbitkan"
	}
	response.Success(c, http.StatusOK, message, p)
}

// Delete godoc
// @Summary      Hapus artikel milik penulis
// @Tags         Posts
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "ID artikel"
// @Success      200  {object}  response.Body
// @Failure      404  {object}  response.Body
// @Router       /api/posts/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(middleware.UserID(c), id); err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil dihapus", nil)
}

// AdminUsers godoc
// @Summary      Daftar seluruh pengguna beserta total artikel
// @Tags         Admin
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Body{data=[]UserSummary}
// @Failure      403  {object}  response.Body
// @Router       /api/admin/users [get]
func (h *Handler) AdminUsers(c *gin.Context) {
	users, err := h.service.ListUsers()
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Daftar pengguna berhasil diambil", users)
}

// AdminPosts godoc
// @Summary      Monitoring seluruh artikel dari semua penulis
// @Tags         Admin
// @Produce      json
// @Security     BearerAuth
// @Param        q         query     string  false  "Kata kunci pencarian"
// @Param        category  query     string  false  "Filter kategori"
// @Success      200       {object}  response.Body{data=[]PostResponse}
// @Failure      403       {object}  response.Body
// @Router       /api/admin/posts [get]
func (h *Handler) AdminPosts(c *gin.Context) {
	posts, err := h.service.ListAll(bindFilter(c))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Daftar seluruh artikel berhasil diambil", posts)
}

// AdminUserPosts godoc
// @Summary      Daftar artikel milik pengguna tertentu
// @Tags         Admin
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "ID pengguna"
// @Success      200  {object}  response.Body{data=[]PostResponse}
// @Failure      404  {object}  response.Body
// @Router       /api/admin/users/{id}/posts [get]
func (h *Handler) AdminUserPosts(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	posts, err := h.service.ListByUser(id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Daftar artikel pengguna berhasil diambil", posts)
}
