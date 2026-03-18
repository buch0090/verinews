package llm

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

const (
	DefaultModel   = openai.GPT4oMini
	MaxContentRune = 12000 // ~3000 tokens — keeps costs predictable
)

type Client struct {
	oc *openai.Client
}

func New(apiKey string) *Client {
	return &Client{oc: openai.NewClient(apiKey)}
}

// JSON sends a chat completion request and unmarshals the response into dst.
// The model is instructed to return only valid JSON matching the schema
// described in systemPrompt.
func (c *Client) JSON(ctx context.Context, systemPrompt, userPrompt string, dst any) error {
	resp, err := c.oc.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: DefaultModel,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Temperature: 0.2, // low temp for structured analytical output
	})
	if err != nil {
		return fmt.Errorf("openai request: %w", err)
	}

	content := resp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(content), dst); err != nil {
		return fmt.Errorf("unmarshaling response: %w\nraw: %s", err, content)
	}
	return nil
}

// Trim caps content length before sending to the LLM to control token cost.
func Trim(content string) string {
	runes := []rune(content)
	if len(runes) <= MaxContentRune {
		return content
	}
	return string(runes[:MaxContentRune])
}
