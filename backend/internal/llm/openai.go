package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
)

type OpenAIClient struct{}

type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

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

	reqBody := openaiRequest{
		Model: "gpt-3.5-turbo",
		Messages: []openaiMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API returned status %d", resp.StatusCode)
	}

	var apiResp openaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, errors.New("no choices in response")
	}

	content := apiResp.Choices[0].Message.Content

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