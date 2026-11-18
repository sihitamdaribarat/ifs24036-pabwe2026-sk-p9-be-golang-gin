package ai

import (
	"errors"
	"net/http"

	"app/internal/features/post"
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
// @Description  Menelusuri topik ke Search Engine (opsional), lalu mengirim prompt + konteks ke OpenAI Chat Completions untuk menghasilkan judul, kategori, excerpt, konten Markdown dan daftar sumber rujukan.
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      GenerateRequest  true  "Topik artikel"
// @Success      200   {object}  response.Body{data=GeneratedArticle}
// @Failure      400   {object}  response.Body
// @Failure      502   {object}  response.Body
// @Failure      503   {object}  response.Body
// @Router       /api/posts/ai-generate [post]
func (h *Handler) Generate(c *gin.Context) {
	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Topik artikel wajib diisi")
		return
	}

	article, err := h.service.Generate(c.Request.Context(), req)
	switch {
	case err == nil:
		response.Success(c, http.StatusOK, "Draf artikel AI berhasil dibuat", article)
	case errors.Is(err, post.ErrInvalidCategory):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNotConfigured):
		response.Error(c, http.StatusServiceUnavailable, err.Error())
	default:
		response.Error(c, http.StatusBadGateway, err.Error())
	}
}
