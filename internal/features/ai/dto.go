// Package ai menyediakan AI Blog Generator yang terintegrasi dengan Search Engine
// dan OpenAI Chat Completions.
package ai

// GenerateRequest adalah payload permintaan pembuatan draf artikel oleh AI.
type GenerateRequest struct {
	Topic            string `json:"topic" binding:"required,max=500" example:"Tren kecerdasan buatan di dunia pendidikan 2026"`
	Category         string `json:"category" example:"Teknologi"`
	Tone             string `json:"tone" binding:"max=100" example:"informatif dan santai"`
	IncludeWebSearch bool   `json:"include_web_search" example:"true"`
}

// SearchResult adalah satu sumber rujukan hasil penelusuran mesin pencari.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// GeneratedArticle adalah draf artikel hasil AI beserta sumber rujukannya.
type GeneratedArticle struct {
	Title         string         `json:"title"`
	Category      string         `json:"category"`
	Excerpt       string         `json:"excerpt"`
	Content       string         `json:"content"`
	Sources       []SearchResult `json:"sources"`
	WebSearchUsed bool           `json:"web_search_used"`
	SearchWarning string         `json:"search_warning,omitempty"`
	GeneratedBy   string         `json:"generated_by" example:"openai"`
}
