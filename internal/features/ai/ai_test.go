package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"app/internal/features/post"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errFake = errors.New("fake error")

// ---------- Search Engine client ----------

func TestSearchClientSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "golang gin", r.URL.Query().Get("q"))
		assert.Equal(t, "3", r.URL.Query().Get("limit"))
		assert.Equal(t, "id", r.URL.Query().Get("lang"))
		_, _ = w.Write([]byte(`{"status":"success","data":{"results":[
			{"title":"Gin Web Framework","url":"https://gin-gonic.com","snippet":"Gin is fast"},
			{"name":"Go","link":"https://go.dev","description":"The Go language"}
		]}}`))
	}))
	defer srv.Close()

	client := NewSearchClient(srv.URL+"?lang=id", srv.Client())
	results, err := client.Search(context.Background(), "golang gin", 3)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, SearchResult{Title: "Gin Web Framework", URL: "https://gin-gonic.com", Snippet: "Gin is fast"}, results[0])
	assert.Equal(t, "Go", results[1].Title)
	assert.Equal(t, "https://go.dev", results[1].URL)
}

func TestSearchClientErrors(t *testing.T) {
	ctx := context.Background()

	_, err := NewSearchClient("://bad-url", http.DefaultClient).Search(ctx, "q", 1)
	assert.Error(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") == "status" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("not json"))
	}))
	_, err = NewSearchClient(srv.URL, srv.Client()).Search(ctx, "status", 1)
	assert.ErrorContains(t, err, "status 500")

	_, err = NewSearchClient(srv.URL, srv.Client()).Search(ctx, "json", 1)
	assert.ErrorContains(t, err, "tidak valid")

	srv.Close()
	_, err = NewSearchClient(srv.URL, http.DefaultClient).Search(ctx, "q", 1)
	assert.Error(t, err)
}

func TestParseSearchResults(t *testing.T) {
	var raw any
	require.NoError(t, json.Unmarshal([]byte(`[
		"bukan objek",
		{"title":"tanpa url"},
		{"url":"https://a.com","content":"`+strings.Repeat("x", MaxSnippetLength+10)+`"},
		{"title":"B","href":"https://b.com","text":"teks b"},
		{"title":"C","url":"https://c.com"}
	]`), &raw))

	results := ParseSearchResults(raw, 2)
	require.Len(t, results, 2)
	assert.Equal(t, "https://a.com", results[0].Title)
	assert.True(t, strings.HasSuffix(results[0].Snippet, "..."))
	assert.Equal(t, "teks b", results[1].Snippet)

	for _, body := range []string{
		`{"results":[{"title":"R","url":"https://r.com"}]}`,
		`{"items":[{"title":"I","url":"https://i.com"}]}`,
		`{"data":[{"title":"D","url":"https://d.com"}]}`,
		`{"organic":[{"title":"O","url":"https://o.com"}]}`,
		`{"webPages":{"value":[{"title":"W","url":"https://w.com"}]}}`,
	} {
		require.NoError(t, json.Unmarshal([]byte(body), &raw))
		assert.Len(t, ParseSearchResults(raw, 5), 1, body)
	}

	assert.Empty(t, ParseSearchResults(map[string]any{"message": "kosong"}, 5))
	assert.Empty(t, ParseSearchResults(nil, 5))
}

// ---------- OpenAI Chat Completions client ----------

func TestChatClientSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		var req chatRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "model-x", req.Model)
		assert.Len(t, req.Messages, 2)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"halo"}}]}`))
	}))
	defer srv.Close()

	client := NewChatClient(srv.URL+"/v1/", "secret", "model-x", srv.Client())
	out, err := client.Complete(context.Background(), "sys", "user")
	require.NoError(t, err)
	assert.Equal(t, "halo", out)
}

func TestChatClientErrors(t *testing.T) {
	ctx := context.Background()

	_, err := NewChatClient("http://x", "", "m", http.DefaultClient).Complete(ctx, "s", "u")
	assert.ErrorIs(t, err, ErrNotConfigured)

	_, err = NewChatClient("http://bad\x7f", "k", "m", http.DefaultClient).Complete(ctx, "s", "u")
	assert.Error(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/unauthorized/chat/completions":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"Invalid API key provided."}}`))
		case "/plain-error/chat/completions":
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`upstream down`))
		case "/invalid/chat/completions":
			_, _ = w.Write([]byte(`not json`))
		default:
			_, _ = w.Write([]byte(`{"choices":[]}`))
		}
	}))
	defer srv.Close()

	_, err = NewChatClient(srv.URL+"/unauthorized", "k", "m", srv.Client()).Complete(ctx, "s", "u")
	assert.ErrorContains(t, err, "Invalid API key provided.")

	_, err = NewChatClient(srv.URL+"/plain-error", "k", "m", srv.Client()).Complete(ctx, "s", "u")
	assert.ErrorContains(t, err, "502")

	_, err = NewChatClient(srv.URL+"/invalid", "k", "m", srv.Client()).Complete(ctx, "s", "u")
	assert.ErrorContains(t, err, "tidak valid")

	_, err = NewChatClient(srv.URL+"/empty", "k", "m", srv.Client()).Complete(ctx, "s", "u")
	assert.ErrorContains(t, err, "tidak mengembalikan konten")

	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	_, err = NewChatClient(closed.URL, "k", "m", http.DefaultClient).Complete(ctx, "s", "u")
	assert.Error(t, err)
}

// ---------- Helper prompt & parser ----------

func TestBuildUserPrompt(t *testing.T) {
	p := BuildUserPrompt("AI", "Teknologi", "", nil)
	assert.Contains(t, p, "Topik artikel: AI")
	assert.Contains(t, p, "Tidak ada hasil penelusuran web")
	assert.NotContains(t, p, "Gaya bahasa")

	p = BuildUserPrompt("AI", "Teknologi", "santai", []SearchResult{{Title: "S1", URL: "https://s1.com", Snippet: "isi"}})
	assert.Contains(t, p, "Gaya bahasa: santai")
	assert.Contains(t, p, "[1] S1")
	assert.Contains(t, p, "URL: https://s1.com")
}

func TestParseArticle(t *testing.T) {
	a, err := ParseArticle("```json\n{\"title\":\"T\",\"category\":\"Opini\",\"excerpt\":\"E\",\"content\":\"## Isi\"}\n```")
	require.NoError(t, err)
	assert.Equal(t, "T", a.Title)
	assert.Equal(t, "## Isi", a.Content)

	_, err = ParseArticle("tidak ada json")
	assert.Error(t, err)
	_, err = ParseArticle("} terbalik {")
	assert.Error(t, err)
	_, err = ParseArticle(`{"title": invalid}`)
	assert.ErrorContains(t, err, "tidak valid")
	_, err = ParseArticle(`{"title":"T","content":"  "}`)
	assert.ErrorContains(t, err, "kosong")
}

func TestAppendSources(t *testing.T) {
	sources := []SearchResult{{Title: "A", URL: "https://a.com"}, {Title: "B", URL: "https://b.com"}}
	assert.Equal(t, "isi", AppendSources("isi", nil))
	assert.Equal(t, "isi\n## Sumber Rujukan", AppendSources("isi\n## Sumber Rujukan", sources))

	out := AppendSources("isi\n\n", sources)
	assert.Equal(t, "isi\n\n## Sumber Rujukan\n\n1. [A](https://a.com)\n2. [B](https://b.com)\n", out)
}

// ---------- Service ----------

type fakeSearcher struct {
	results []SearchResult
	err     error
	called  bool
}

func (f *fakeSearcher) Search(_ context.Context, _ string, _ int) ([]SearchResult, error) {
	f.called = true
	return f.results, f.err
}

type fakeCompleter struct {
	out        string
	err        error
	lastPrompt string
}

func (f *fakeCompleter) Complete(_ context.Context, _, user string) (string, error) {
	f.lastPrompt = user
	return f.out, f.err
}

const articleJSON = `{"title":"Judul AI","category":"pendidikan","excerpt":" Ringkas ","content":"## Isi artikel"}`

func TestGenerateWithWebSearch(t *testing.T) {
	searcher := &fakeSearcher{results: []SearchResult{{Title: "Sumber", URL: "https://sumber.com", Snippet: "fakta"}}}
	completer := &fakeCompleter{out: articleJSON}
	svc := NewService(searcher, completer)

	a, err := svc.Generate(context.Background(), GenerateRequest{Topic: " AI Sekolah ", Category: "Teknologi", IncludeWebSearch: true})
	require.NoError(t, err)
	assert.True(t, searcher.called)
	assert.Contains(t, completer.lastPrompt, "https://sumber.com")
	assert.Equal(t, "Judul AI", a.Title)
	assert.Equal(t, "Pendidikan", a.Category)
	assert.Equal(t, "Ringkas", a.Excerpt)
	assert.Contains(t, a.Content, "## Sumber Rujukan")
	assert.Len(t, a.Sources, 1)
	assert.True(t, a.WebSearchUsed)
	assert.Empty(t, a.SearchWarning)
}

func TestGenerateWithoutWebSearch(t *testing.T) {
	searcher := &fakeSearcher{}
	completer := &fakeCompleter{out: `{"title":"","category":"Olahraga","content":"isi"}`}
	svc := NewService(searcher, completer)

	a, err := svc.Generate(context.Background(), GenerateRequest{Topic: "Topik", Category: "Bisnis"})
	require.NoError(t, err)
	assert.False(t, searcher.called)
	assert.Equal(t, "Topik", a.Title)
	assert.Equal(t, "Bisnis", a.Category)
	assert.Empty(t, a.Sources)
	assert.False(t, a.WebSearchUsed)

	completer.out = `{"title":"X","content":"isi"}`
	a, err = svc.Generate(context.Background(), GenerateRequest{Topic: "Topik", Category: "Opini"})
	require.NoError(t, err)
	assert.Equal(t, "Opini", a.Category)
}

func TestGenerateSearchFailureIsGraceful(t *testing.T) {
	svc := NewService(&fakeSearcher{err: errFake}, &fakeCompleter{out: articleJSON})
	a, err := svc.Generate(context.Background(), GenerateRequest{Topic: "AI", IncludeWebSearch: true})
	require.NoError(t, err)
	assert.False(t, a.WebSearchUsed)
	assert.Contains(t, a.SearchWarning, "fake error")
	assert.Empty(t, a.Sources)
}

func TestGenerateErrors(t *testing.T) {
	ctx := context.Background()

	_, err := NewService(&fakeSearcher{}, &fakeCompleter{}).Generate(ctx, GenerateRequest{Topic: "x", Category: "Olahraga"})
	assert.ErrorIs(t, err, post.ErrInvalidCategory)

	_, err = NewService(&fakeSearcher{}, &fakeCompleter{err: ErrNotConfigured}).Generate(ctx, GenerateRequest{Topic: "x"})
	assert.ErrorIs(t, err, ErrNotConfigured)

	_, err = NewService(&fakeSearcher{}, &fakeCompleter{err: errFake}).Generate(ctx, GenerateRequest{Topic: "x"})
	assert.ErrorIs(t, err, ErrGenerationFailed)

	_, err = NewService(&fakeSearcher{}, &fakeCompleter{out: "bukan json"}).Generate(ctx, GenerateRequest{Topic: "x"})
	assert.ErrorIs(t, err, ErrGenerationFailed)
}

// ---------- Handler ----------

func callGenerate(svc *Service, body string) (int, map[string]any) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/posts/ai-generate", NewHandler(svc).Generate)

	req := httptest.NewRequest(http.MethodPost, "/api/posts/ai-generate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var parsed map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &parsed)
	return w.Code, parsed
}

func TestHandlerGenerate(t *testing.T) {
	ok := NewService(&fakeSearcher{}, &fakeCompleter{out: articleJSON})

	code, body := callGenerate(ok, `{"topic":"AI","include_web_search":false}`)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "success", body["status"])
	assert.Equal(t, "Judul AI", body["data"].(map[string]any)["title"])

	code, _ = callGenerate(ok, `{}`)
	assert.Equal(t, http.StatusBadRequest, code)

	code, _ = callGenerate(ok, `{"topic":"AI","category":"Olahraga"}`)
	assert.Equal(t, http.StatusBadRequest, code)

	code, _ = callGenerate(NewService(&fakeSearcher{}, &fakeCompleter{err: ErrNotConfigured}), `{"topic":"AI"}`)
	assert.Equal(t, http.StatusServiceUnavailable, code)

	code, body = callGenerate(NewService(&fakeSearcher{}, &fakeCompleter{err: errFake}), `{"topic":"AI"}`)
	assert.Equal(t, http.StatusBadGateway, code)
	assert.Equal(t, "error", body["status"])
}
