// Package deepseek implements an OpenAI-compatible client for DeepSeek's chat API,
// satisfying the ollama.GenerateClientWithTools interface so it plugs directly
// into the tsll workflow tool loop.
package openapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pedrobelmino/tui-sdd-open-api/internal/ollama"
)

const (
	DefaultBaseURL = "https://api.deepseek.com"
	DefaultModel   = "deepseek-chat"
	apiTimeout     = 10 * time.Minute
)

// Client is a DeepSeek API client that implements GenerateClientWithTools.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// NewClient creates a DeepSeek client. apiKey is required. When baseURL is empty,
// the default DeepSeek API endpoint is used. model defaults to "deepseek-chat".
func NewClient(apiKey, baseURL, model string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if model == "" {
		model = DefaultModel
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		http:    &http.Client{},
	}
}

// -- metadata Client interface -------------------------------------------------

func (c *Client) Tags(_ context.Context) ([]ollama.TagModel, error) {
	return []ollama.TagModel{{Name: c.model}}, nil
}

func (c *Client) Ps(_ context.Context) ([]ollama.RunningModel, error) {
	return nil, nil
}

func (c *Client) Reachable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/models", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

// -- generation ----------------------------------------------------------------

// deepseekChatRequest is the OpenAI-compatible chat payload.
type deepseekChatRequest struct {
	Model    string              `json:"model"`
	Messages []deepseekMessage   `json:"messages"`
	Stream   bool                `json:"stream"`
}

type deepseekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// deepseekChatResponse is a single chunk or final response from the streaming API.
type deepseekChatResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta,omitempty"`
		Message struct {
			Content string `json:"content"`
		} `json:"message,omitempty"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) Chat(ctx context.Context, req ollama.ChatRequest) (string, ollama.TokenUsage, error) {
	body, err := json.Marshal(deepseekChatRequest{
		Model:    c.model,
		Stream:   false,
		Messages: toDeepSeekMsgs(req.Messages),
	})
	if err != nil {
		return "", ollama.TokenUsage{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", ollama.TokenUsage{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", ollama.TokenUsage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", ollama.TokenUsage{}, fmt.Errorf("deepseek: %d: %s", resp.StatusCode, string(b))
	}

	var out deepseekChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", ollama.TokenUsage{}, err
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", ollama.TokenUsage{}, fmt.Errorf("deepseek: %s", out.Error.Message)
	}

	text := ""
	if len(out.Choices) > 0 {
		text = out.Choices[0].Message.Content
	}
	usage := ollama.TokenUsage{}
	if out.Usage != nil {
		usage.PromptTokens = out.Usage.PromptTokens
		usage.CompletionTokens = out.Usage.CompletionTokens
	}
	return text, usage, nil
}

func (c *Client) ChatStream(ctx context.Context, req ollama.ChatRequest, onChunk func(string)) (string, ollama.TokenUsage, error) {
	body, err := json.Marshal(deepseekChatRequest{
		Model:    c.model,
		Stream:   true,
		Messages: toDeepSeekMsgs(req.Messages),
	})
	if err != nil {
		return "", ollama.TokenUsage{}, err
	}

	streamCtx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(streamCtx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", ollama.TokenUsage{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", ollama.TokenUsage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", ollama.TokenUsage{}, fmt.Errorf("deepseek: %d: %s", resp.StatusCode, string(b))
	}

	var full strings.Builder
	var usage ollama.TokenUsage
	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				if err != nil {
					break
				}
				continue
			}
			// SSE data: lines start with "data: "
			raw := string(line)
			if !strings.HasPrefix(raw, "data: ") {
				if err != nil {
					break
				}
				continue
			}
			payload := strings.TrimPrefix(raw, "data: ")
			if payload == "[DONE]" {
				break
			}

			var chunk deepseekChatResponse
			if json.Unmarshal([]byte(payload), &chunk) != nil {
				continue
			}
			if chunk.Error != nil && chunk.Error.Message != "" {
				return full.String(), usage, fmt.Errorf("deepseek: %s", chunk.Error.Message)
			}
			if len(chunk.Choices) > 0 {
				content := chunk.Choices[0].Delta.Content
				if content != "" {
					full.WriteString(content)
					if onChunk != nil {
						onChunk(content)
					}
				}
				if chunk.Usage != nil {
					usage.PromptTokens = chunk.Usage.PromptTokens
					usage.CompletionTokens = chunk.Usage.CompletionTokens
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return full.String(), usage, err
		}
	}
	return full.String(), usage, nil
}

// -- tools --------------------------------------------------------------------

func (c *Client) ChatWithTools(ctx context.Context, msgs []ollama.ChatMessageWithTools, tools []ollama.ToolDef, exec ollama.ToolExecutor, onChunk func(string)) (string, ollama.TokenUsage, error) {
	plain := make([]ollama.ChatMessage, len(msgs))
	for i, m := range msgs {
		plain[i] = ollama.ChatMessage{Role: m.Role, Content: m.Content}
	}
	return c.ChatStream(ctx, ollama.ChatRequest{Model: c.model, Messages: plain}, onChunk)
}

// -- helpers ------------------------------------------------------------------

func toDeepSeekMsgs(msgs []ollama.ChatMessage) []deepseekMessage {
	out := make([]deepseekMessage, len(msgs))
	for i, m := range msgs {
		out[i] = deepseekMessage{Role: m.Role, Content: m.Content}
	}
	return out
}
