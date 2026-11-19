package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"app/internal/features/post"
)

// SearchLimit adalah jumlah maksimal sumber rujukan web yang dipakai.
const SearchLimit = 5

// ErrEmptyTopic dikembalikan jika topik hanya berisi spasi.
var ErrEmptyTopic = errors.New("topic wajib diisi (topik artikel)")

// Searcher adalah kontrak penelusuran mesin pencari.
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

// Completer adalah kontrak model bahasa (chat completions).
type Completer interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// Service mengorkestrasi Search Engine + OpenAI untuk menghasilkan draf artikel.
type Service struct {
	searcher  Searcher
	completer Completer
}

// NewService membuat Service AI.
func NewService(searcher Searcher, completer Completer) *Service {
	return &Service{searcher: searcher, completer: completer}
}

const systemPrompt = `Anda adalah penulis blog profesional berbahasa Indonesia untuk platform Delcom Blog.
Tulis artikel orisinal, akurat, terstruktur dan mudah dibaca dalam format Markdown
(gunakan heading ##, paragraf, daftar, dan penekanan seperlunya; jangan gunakan heading # di konten).
Jika diberikan hasil penelusuran web, gunakan sebagai fakta terkini dan cantumkan sitasi bernomor seperti [1], [2]
sesuai nomor sumber, lalu akhiri konten dengan bagian "## Sumber Rujukan".
Balas HANYA dengan satu objek JSON valid tanpa teks lain, dengan format:
{"title": "...", "category": "...", "excerpt": "...", "content": "..."}
- title: judul menarik maksimal 120 karakter
- category: salah satu dari Teknologi, Pendidikan, Gaya Hidup, Bisnis, Opini, Umum
- excerpt: ringkasan pemikat 1-2 kalimat
- content: isi artikel lengkap dalam Markdown (sekitar 300-500 kata agar respons tetap cepat)`

// BuildUserPrompt menyusun prompt topik beserta konteks hasil penelusuran mesin pencari.
func BuildUserPrompt(topic, category, tone string, sources []SearchResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Topik artikel: %s\n", topic)
	fmt.Fprintf(&b, "Kategori yang diinginkan: %s\n", category)
	if tone != "" {
		fmt.Fprintf(&b, "Gaya bahasa: %s\n", tone)
	}

	if len(sources) == 0 {
		b.WriteString("\nTidak ada hasil penelusuran web. Tulis berdasarkan pengetahuan umum yang akurat dan jangan mengarang sitasi.\n")
		return b.String()
	}

	b.WriteString("\nHasil penelusuran mesin pencari (gunakan sebagai konteks dan sitasi):\n")
	for i, s := range sources {
		fmt.Fprintf(&b, "[%d] %s\nURL: %s\nRingkasan: %s\n\n", i+1, s.Title, s.URL, s.Snippet)
	}
	return b.String()
}

// ParseArticle mengurai JSON artikel dari jawaban AI (mendukung pembungkus ```json ... ```).
func ParseArticle(raw string) (*GeneratedArticle, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end < start {
		return nil, errors.New("jawaban AI tidak berisi JSON")
	}

	var article GeneratedArticle
	if err := json.Unmarshal([]byte(raw[start:end+1]), &article); err != nil {
		return nil, fmt.Errorf("JSON artikel AI tidak valid: %w", err)
	}
	if strings.TrimSpace(article.Content) == "" {
		return nil, errors.New("konten artikel AI kosong")
	}
	return &article, nil
}

// AppendSources menambahkan bagian "Sumber Rujukan" ke konten jika belum ada.
func AppendSources(content string, sources []SearchResult) string {
	if len(sources) == 0 || strings.Contains(content, "Sumber Rujukan") {
		return content
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(content, "\n"))
	b.WriteString("\n\n## Sumber Rujukan\n\n")
	for i, s := range sources {
		fmt.Fprintf(&b, "%d. [%s](%s)\n", i+1, s.Title, s.URL)
	}
	return b.String()
}

// Generate menghasilkan draf artikel utuh. Jika IncludeWebSearch aktif, topik ditelusuri
// terlebih dahulu ke Search Engine; kegagalan penelusuran tidak menggagalkan proses.
// Jika OpenAI belum dikonfigurasi, gagal, atau jawabannya tidak valid, draf disusun oleh
// penulis lokal (BuildLocalArticle) sehingga endpoint tetap menghasilkan artikel.
func (s *Service) Generate(ctx context.Context, req GenerateRequest) (*GeneratedArticle, error) {
	topic := strings.TrimSpace(req.Topic)
	if topic == "" {
		return nil, ErrEmptyTopic
	}
	category, err := post.NormalizeCategory(req.Category)
	if err != nil {
		return nil, err
	}

	sources := []SearchResult{}
	warning := ""
	if req.IncludeWebSearch {
		results, err := s.searcher.Search(ctx, topic, SearchLimit)
		if err != nil {
			warning = "Penelusuran mesin pencari gagal: " + err.Error()
		} else {
			sources = results
		}
	}

	tone := strings.TrimSpace(req.Tone)
	var article *GeneratedArticle
	raw, err := s.completer.Complete(ctx, systemPrompt, BuildUserPrompt(topic, category, tone, sources))
	if err == nil {
		article, err = ParseArticle(raw)
	}
	generatedBy := GeneratorOpenAI
	if err != nil {
		log.Printf("[WARN] AI generator memakai penulis lokal: %v", err)
		article = BuildLocalArticle(topic, category, tone, sources)
		generatedBy = GeneratorLocal
	}

	if strings.TrimSpace(article.Title) == "" {
		article.Title = topic
	}
	if normalized, err := post.NormalizeCategory(article.Category); err == nil && article.Category != "" {
		article.Category = normalized
	} else {
		article.Category = category
	}
	article.Title = strings.TrimSpace(article.Title)
	article.Excerpt = strings.TrimSpace(article.Excerpt)
	article.Content = AppendSources(article.Content, sources)
	article.Sources = sources
	article.WebSearchUsed = req.IncludeWebSearch && warning == ""
	article.SearchWarning = warning
	article.GeneratedBy = generatedBy
	return article, nil
}
