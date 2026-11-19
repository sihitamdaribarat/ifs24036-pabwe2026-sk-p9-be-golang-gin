package auth

import (
	"errors"
	"net/http"

	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
)

// Handler adalah HTTP handler endpoint /api/auth/*.
type Handler struct {
	service *Service
}

// NewHandler membuat Handler autentikasi.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// statusFor memetakan error domain ke HTTP status code.
func statusFor(err error) int {
	switch {
	case errors.Is(err, ErrInvalidUsername):
		return http.StatusBadRequest
	case errors.Is(err, ErrUsernameTaken), errors.Is(err, ErrEmailTaken):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidCredentials):
		return http.StatusUnauthorized
	case errors.Is(err, ErrUserNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func respondError(c *gin.Context, err error) {
	code := statusFor(err)
	if code == http.StatusInternalServerError {
		response.InternalError(c, err)
		return
	}
	response.Fail(c, code, err.Error())
}

// Register godoc
// @Summary      Registrasi penulis baru
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      RegisterRequest  true  "Data registrasi"
// @Success      201   {object}  response.Body{data=User}
// @Failure      400   {object}  response.Body
// @Failure      409   {object}  response.Body
// @Router       /api/auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data registrasi tidak valid: "+err.Error())
		return
	}

	user, err := h.service.Register(req)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusCreated, "Registrasi berhasil", user)
}

// Login godoc
// @Summary      Login dan dapatkan token JWT
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        body  body      LoginRequest  true  "Kredensial"
// @Success      200   {object}  response.Body{data=LoginResponse}
// @Failure      400   {object}  response.Body
// @Failure      401   {object}  response.Body
// @Router       /api/auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Identifier() == "" {
		response.Fail(c, http.StatusBadRequest, "Username dan password wajib diisi")
		return
	}

	result, err := h.service.Login(req)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Login berhasil", result)
}

// Logout godoc
// @Summary      Logout sesi pengguna
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Body
// @Failure      401  {object}  response.Body
// @Router       /api/auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	middleware.RevokeToken(c.GetString(middleware.ContextToken))
	response.Success(c, http.StatusOK, "Logout berhasil", nil)
}

// GetProfile godoc
// @Summary      Profil penulis yang sedang login
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  response.Body{data=User}
// @Failure      401  {object}  response.Body
// @Router       /api/auth/profile [get]
func (h *Handler) GetProfile(c *gin.Context) {
	user, err := h.service.Profile(middleware.UserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Profil berhasil diambil", user)
}

// UpdateProfile godoc
// @Summary      Perbarui profil penulis yang sedang login
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      UpdateProfileRequest  true  "Data profil"
// @Success      200   {object}  response.Body{data=User}
// @Failure      400   {object}  response.Body
// @Failure      409   {object}  response.Body
// @Router       /api/auth/profile [put]
func (h *Handler) UpdateProfile(c *gin.Context) {
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data profil tidak valid: "+err.Error())
		return
	}

	user, err := h.service.UpdateProfile(middleware.UserID(c), req)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Profil berhasil diperbarui", user)
}
