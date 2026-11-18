package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// MaxSnippetLength membatasi panjang snippet agar prompt AI tetap ringkas.
const MaxSnippetLength = 500

// SearchClient adalah client HTTP untuk Search Engine (GET endpoint?q=...&limit=...).
type SearchClient struct {
	endpoint string
	http     *http.Client
}

// NewSearchClient membuat SearchClient.
func NewSearchClient(endpoint string, httpClient *http.Client) *SearchClient {
	return &SearchClient{endpoint: endpoint, http: httpClient}
}

// Search melakukan penelusuran topik ke Search Engine dan mengembalikan maksimal limit hasil.
func (c *SearchClient) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("limit", strconv.Itoa(limit))

	separator := "?"
	if strings.Contains(c.endpoint, "?") {
		separator = "&"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint+separator+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("search engine merespons status %d", resp.StatusCode)
	}

	var raw any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("respons search engine tidak valid: %w", err)
	}
	return ParseSearchResults(raw, limit), nil
}

// ParseSearchResults menormalkan berbagai bentuk respons JSON mesin pencari
// (misal {data:{results:[...]}}, {results:[...]}, {items:[...]} atau array langsung).
func ParseSearchResults(raw any, limit int) []SearchResult {
	results := []SearchResult{}
	for _, item := range findItems(raw) {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		link := firstString(m, "url", "link", "href")
		if link == "" {
			continue
		}
		title := firstString(m, "title", "name")
		if title == "" {
			title = link
		}
		results = append(results, SearchResult{
			Title:   title,
			URL:     link,
			Snippet: truncate(firstString(m, "snippet", "description", "content", "body", "text"), MaxSnippetLength),
		})
		if len(results) == limit {
			break
		}
	}
	return results
}

func findItems(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case map[string]any:
		for _, key := range []string{"results", "items", "data", "organic", "webPages", "value"} {
			if items := findItems(t[key]); len(items) > 0 {
				return items
			}
		}
	}
	return nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s, ok := m[key].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
