// Package llm provides LLM provider implementations.
package llm

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/genai"
)

// GeminiProvider implements LLMProvider using Google GenAI Gemini.
type GeminiProvider struct {
	client *genai.Client
	model  string
}

// GeminiConfig holds configuration for the Gemini provider.
type GeminiConfig struct {
	APIKey string // If empty, uses GOOGLE_API_KEY env var
	Model  string // e.g., "gemini-3-pro"
}

// DefaultGeminiConfig returns default configuration.
func DefaultGeminiConfig() GeminiConfig {
	return GeminiConfig{}
}

// NewGeminiProvider creates a new Gemini provider.
func NewGeminiProvider(ctx context.Context, cfg GeminiConfig) (*GeminiProvider, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY not set")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	model := cfg.Model
	if model == "" {
		model = os.Getenv("GOOGLE_MODEL")
	}
	if model == "" {
		model = "gemini-3-pro"
	}

	return &GeminiProvider{
		client: client,
		model:  model,
	}, nil
}

// Generate produces a response from Gemini.
func (p *GeminiProvider) Generate(ctx context.Context, prompt string) (string, error) {
	resp, err := p.client.Models.GenerateContent(ctx, p.model, genai.Text(prompt), nil)
	if err != nil {
		return "", fmt.Errorf("gemini generate failed: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", fmt.Errorf("no response from gemini")
	}

	// Extract text from response
	var result string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part != nil && part.Text != "" {
			result += part.Text
		}
	}

	return result, nil
}

// GenerateWithConfig produces a response with custom generation config.
func (p *GeminiProvider) GenerateWithConfig(ctx context.Context, prompt string, config *genai.GenerateContentConfig) (string, error) {
	resp, err := p.client.Models.GenerateContent(ctx, p.model, genai.Text(prompt), config)
	if err != nil {
		return "", fmt.Errorf("gemini generate failed: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", fmt.Errorf("no response from gemini")
	}

	var result string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part != nil && part.Text != "" {
			result += part.Text
		}
	}

	return result, nil
}

// Close closes the provider.
func (p *GeminiProvider) Close() {
	// Client doesn't need explicit close in current SDK
}

// Model returns the model name.
func (p *GeminiProvider) Model() string {
	return p.model
}
