// Package response menyediakan format respons JSON standar {status, message, data?}.
package response

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Body adalah struktur respons JSON standar seluruh endpoint API.
type Body struct {
	Status  string `json:"status" example:"success"`
	Message string `json:"message" example:"Berhasil"`
	Data    any    `json:"data,omitempty"`
}

// Success mengirim respons sukses (status "success") beserta data opsional.
func Success(c *gin.Context, code int, message string, data any) {
	c.JSON(code, Body{Status: "success", Message: message, Data: data})
}

// Fail mengirim respons kegagalan dari sisi klien (status "fail"), misal 400/401/404.
func Fail(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, Body{Status: "fail", Message: message})
}

// InternalError mencatat error asli ke log server (terlihat di tab Logs platform) lalu
// mengirim respons 500 generik tanpa membocorkan detail internal ke klien.
func InternalError(c *gin.Context, err error) {
	log.Printf("[ERROR] %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	Error(c, http.StatusInternalServerError, "Terjadi kesalahan pada server")
}

// Error mengirim respons kesalahan dari sisi server (status "error"), misal 500/502.
func Error(c *gin.Context, code int, message string) {
	c.AbortWithStatusJSON(code, Body{Status: "error", Message: message})
}
