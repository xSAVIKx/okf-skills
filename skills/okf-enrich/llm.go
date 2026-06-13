package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Generator produces a short natural-language description for a prompt.
type Generator interface {
	Describe(ctx context.Context, prompt string) (string, error)
}

const systemPrompt = "You are a data documentation assistant. Given details about a database table, dataset, file, or directory, write a single concise sentence describing what it represents and its purpose. Respond with only the description sentence, no preamble."

// OpenAIGenerator calls an OpenAI-compatible /chat/completions endpoint.
type OpenAIGenerator struct {
	BaseURL string
	Model   string
	APIKey  string
	Client  *http.Client
}

// NewOpenAIGenerator builds a generator targeting an OpenAI-compatible API.
func NewOpenAIGenerator(baseURL, model, apiKey string) *OpenAIGenerator {
	return &OpenAIGenerator{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Model:   model,
		APIKey:  apiKey,
		Client:  &http.Client{Timeout: 60 * time.Second},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Describe sends the prompt to the chat-completions endpoint and returns the
// generated text.
func (g *OpenAIGenerator) Describe(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:       g.Model,
		Messages:    []chatMessage{{Role: "system", Content: systemPrompt}, {Role: "user", Content: prompt}},
		Temperature: 0.2,
		MaxTokens:   120,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if g.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+g.APIKey)
	}
	resp, err := g.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var cr chatResponse
	if err := json.Unmarshal(data, &cr); err != nil {
		return "", fmt.Errorf("decode LLM response: %w", err)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("LLM error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}
	return strings.TrimSpace(cr.Choices[0].Message.Content), nil
}
