package response

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	var body map[string]any
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

func TestSuccess(t *testing.T) {
	c, w := newContext()
	Success(c, http.StatusOK, "ok", gin.H{"id": 1})

	body := decode(t, w)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", body["status"])
	assert.Equal(t, "ok", body["message"])
	assert.Equal(t, float64(1), body["data"].(map[string]any)["id"])
}

func TestSuccessWithoutData(t *testing.T) {
	c, w := newContext()
	Success(c, http.StatusOK, "ok", nil)

	body := decode(t, w)
	_, hasData := body["data"]
	assert.False(t, hasData)
}

func TestFail(t *testing.T) {
	c, w := newContext()
	Fail(c, http.StatusNotFound, "Endpoint not found")

	body := decode(t, w)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "fail", body["status"])
	assert.Equal(t, "Endpoint not found", body["message"])
	assert.True(t, c.IsAborted())
}

func TestInternalErrorLogsCause(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	c, w := newContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	InternalError(c, errors.New("relation \"users\" does not exist"))

	body := decode(t, w)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "Terjadi kesalahan pada server", body["message"])
	assert.Contains(t, buf.String(), "POST /api/auth/login")
	assert.Contains(t, buf.String(), `relation "users" does not exist`)
}

func TestError(t *testing.T) {
	c, w := newContext()
	Error(c, http.StatusInternalServerError, "boom")

	body := decode(t, w)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "error", body["status"])
	assert.True(t, c.IsAborted())
}
