package ai

import (
	"net/http"

	"app/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler adalah HTTP handler endpoint /api/posts/ai-generate.
type Handler struct {
	service *Service
}

// NewHandler membuat Handler AI.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Generate godoc
// @Summary      Generate draf artikel dengan AI + Search Engine
// @Description  Menelusuri topik ke Search Engine (opsional), lalu mengirim prompt + konteks ke OpenAI Chat Completions untuk menghasilkan judul, kategori, excerpt, konten Markdown dan daftar sumber rujukan. Jika OpenAI belum dikonfigurasi atau gagal, draf disusun oleh penulis lokal (generated_by = "local").
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      GenerateRequest  true  "Topik artikel"
// @Success      200   {object}  response.Body{data=GeneratedArticle}
// @Failure      400   {object}  response.Body
// @Failure      401   {object}  response.Body
// @Router       /api/posts/ai-generate [post]
func (h *Handler) Generate(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Topik artikel wajib diisi")
		return
	}

	// Kegagalan layanan AI sudah ditangani penulis lokal, sehingga error yang tersisa
	// hanya berasal dari input (topik kosong / kategori tidak valid).
	article, err := h.service.Generate(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Draf artikel AI berhasil dibuat", article)
}
