package docs

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/swaggo/swag"
)

func TestSwaggerDocRegistered(t *testing.T) {
	doc, err := swag.ReadDoc(SwaggerInfo.InstanceName())
	assert.NoError(t, err)

	var spec map[string]any
	assert.NoError(t, json.Unmarshal([]byte(doc), &spec))
	assert.Equal(t, "2.0", spec["swagger"])

	paths := spec["paths"].(map[string]any)
	for _, p := range []string{
		"/api/public/posts", "/api/public/posts/{slug}", "/api/public/categories",
		"/api/auth/login", "/api/auth/register", "/api/auth/profile", "/api/auth/logout",
		"/api/posts", "/api/posts/{id}", "/api/posts/stats", "/api/posts/{id}/toggle-publish",
		"/api/posts/ai-generate", "/api/admin/users", "/api/admin/posts", "/api/admin/users/{id}/posts",
	} {
		assert.Contains(t, paths, p)
	}
}

func TestStaticSpecFilesAreValidJSON(t *testing.T) {
	for _, file := range []string{"swagger.json", "api.json"} {
		raw, err := os.ReadFile(file)
		assert.NoError(t, err)
		assert.True(t, json.Valid(raw), file)
	}
}
