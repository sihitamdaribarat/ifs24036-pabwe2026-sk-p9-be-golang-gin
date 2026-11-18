package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrNotConfigured dikembalikan jika OPENAI_KEY / OPENAI_MODEL belum diatur.
var ErrNotConfigured = errors.New("layanan AI belum dikonfigurasi (OPENAI_KEY / OPENAI_MODEL)")

// ChatClient adalah client HTTP OpenAI Chat Completions (POST {endpoint}/chat/completions).
type ChatClient struct {
	endpoint string
	key      string
	model    string
	http     *http.Client
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

// NewChatClient membuat ChatClient.
func NewChatClient(endpoint, key, model string, httpClient *http.Client) *ChatClient {
	return &ChatClient{
		endpoint: strings.TrimRight(endpoint, "/"),
		key:      key,
		model:    model,
		http:     httpClient,
	}
}

// Complete mengirim system & user prompt ke Chat Completions dan mengembalikan teks jawaban.
func (c *ChatClient) Complete(ctx context.Context, system, user string) (string, error) {
	if c.key == "" || c.model == "" {
		return "", ErrNotConfigured
	}

	body, _ := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: 0.7,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.key)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out chatResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&out)

	if resp.StatusCode >= http.StatusMultipleChoices {
		message := out.Error.Message
		if message == "" {
			message = resp.Status
		}
		return "", fmt.Errorf("layanan AI merespons error: %s", message)
	}
	if decodeErr != nil {
		return "", fmt.Errorf("respons layanan AI tidak valid: %w", decodeErr)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", errors.New("layanan AI tidak mengembalikan konten")
	}
	return out.Choices[0].Message.Content, nil
}
