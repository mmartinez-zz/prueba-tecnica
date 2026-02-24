package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type OpenAIClient struct{}

type openaiResponse struct {
	Output []struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

var ErrRateLimited = errors.New("llm rate limited")

func (c *OpenAIClient) ClassifyTask(ctx context.Context, title, description string) (*TaskClassification, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("OPENAI_API_KEY environment variable not set")
	}

	prompt := fmt.Sprintf(`
You are a task classification assistant.

Return ONLY valid JSON in this exact structure:

{
  "tags": ["string"],
  "priority": "low|medium|high|urgent",
  "category": "bug|feature|improvement|research",
  "summary": "one concise sentence"
}

Task:
Title: %s
Description: %s
`, title, description)

	reqBody := map[string]interface{}{
	"model": "gpt-4.1-mini",
	"input": prompt,
	"response_format": map[string]string{
		"type": "json_object",
	},
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
	ctx,
	http.MethodPost,
	"https://api.openai.com/v1/responses",
	bytes.NewReader(jsonBytes),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error %d: %s", resp.StatusCode, string(body))
	}

	var apiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Output) == 0 ||
		len(apiResp.Output[0].Content) == 0 {
		return nil, errors.New("empty response from OpenAI")
	}

	content := apiResp.Output[0].Content[0].Text

	// Clean potential backtick delimiters
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) > 1 && strings.HasSuffix(lines[len(lines)-1], "```") {
			content = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var result TaskClassification
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal classification: %w", err)
	}

	if len(result.Tags) == 0 {
		return nil, errors.New("no tags returned")
	}

	if strings.TrimSpace(result.Summary) == "" {
		return nil, errors.New("empty summary")
	}

	// Validate fields
	validPriorities := map[string]bool{"low": true, "medium": true, "high": true, "urgent": true}
	if !validPriorities[result.Priority] {
		return nil, errors.New("invalid priority in classification")
	}

	validCategories := map[string]bool{"bug": true, "feature": true, "improvement": true, "research": true}
	if !validCategories[result.Category] {
		return nil, errors.New("invalid category in classification")
	}

	return &result, nil
}