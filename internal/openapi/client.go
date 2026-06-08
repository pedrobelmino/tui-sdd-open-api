// Package openapi implements an OpenAI-compatible chat client (DeepSeek,
// OpenRouter, etc.) that satisfies the ollama.GenerateClientWithTools
// interface so it plugs directly into the sdd-open-api workflow tool loop.
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

// Client is an OpenAI-compatible API client that implements GenerateClientWithTools.
type Client struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// NewClient creates an OpenAPI client. apiKey is required. When baseURL is empty,
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

// -- API types ----------------------------------------------------------------

// openAPIChatRequest is the OpenAI-compatible chat payload.
type openAPIChatRequest struct {
	Model       string            `json:"model"`
	Messages    []openAPIMessage  `json:"messages"`
	Stream      bool              `json:"stream"`
	Tools       []openAPITool     `json:"tools,omitempty"`
	ToolChoice  any               `json:"tool_choice,omitempty"` // "auto" | "none" | {"type":"function","function":{"name":"..."}}
}

type openAPIMessage struct {
	Role         string           `json:"role"`
	Content      string           `json:"content,omitempty"`
	ToolCallID   string           `json:"tool_call_id,omitempty"`
	ToolCalls    []openAPIToolCall `json:"tool_calls,omitempty"`
}

type openAPITool struct {
	Type     string          `json:"type"` // "function"
	Function openAPIFunction `json:"function"`
}

type openAPIFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type openAPIToolCall struct {
	Index    *int              `json:"index,omitempty"`
	ID       string            `json:"id"`
	Type     string            `json:"type"` // "function"
	Function openAPIToolFunction `json:"function"`
}

type openAPIToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// openAPIStreamChunk is a single SSE chunk from the streaming endpoint.
type openAPIStreamChunk struct {
	Choices []struct {
		Index        int              `json:"index"`
		Delta        openStreamDelta  `json:"delta"`
		FinishReason *string          `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type openStreamDelta struct {
	Role      string            `json:"role,omitempty"`
	Content   string            `json:"content,omitempty"`
	ToolCalls []openAPIToolCall `json:"tool_calls,omitempty"`
}

// openAPINonStreamResponse is the non-streaming response body.
type openAPINonStreamResponse struct {
	Choices []struct {
		Index        int              `json:"index"`
		Message      openAPIMessage   `json:"message"`
		FinishReason string           `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// -- Chat (non-streaming) -----------------------------------------------------

func (c *Client) Chat(ctx context.Context, req ollama.ChatRequest) (string, ollama.TokenUsage, error) {
	body, err := json.Marshal(openAPIChatRequest{
		Model:    c.model,
		Stream:   false,
		Messages: toOpenAPIMessages(req.Messages),
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
		return "", ollama.TokenUsage{}, fmt.Errorf("openapi: %d: %s", resp.StatusCode, string(b))
	}

	var out openAPINonStreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", ollama.TokenUsage{}, err
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", ollama.TokenUsage{}, fmt.Errorf("openapi: %s", out.Error.Message)
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

// -- ChatStream ---------------------------------------------------------------

func (c *Client) ChatStream(ctx context.Context, req ollama.ChatRequest, onChunk func(string)) (string, ollama.TokenUsage, error) {
	body, err := json.Marshal(openAPIChatRequest{
		Model:    c.model,
		Stream:   true,
		Messages: toOpenAPIMessages(req.Messages),
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
		return "", ollama.TokenUsage{}, fmt.Errorf("openapi: %d: %s", resp.StatusCode, string(b))
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

			var chunk openAPIStreamChunk
			if json.Unmarshal([]byte(payload), &chunk) != nil {
				continue
			}
			if chunk.Error != nil && chunk.Error.Message != "" {
				return full.String(), usage, fmt.Errorf("openapi: %s", chunk.Error.Message)
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

// -- ChatWithTools (tool-calling loop) ----------------------------------------
//
// Uses native OpenAI-compatible tool_calls:
//  1. Send messages + tool definitions to the API (non-streaming)
//  2. If response includes tool_calls → execute each, append results, loop
//  3. If response has text content → stream it to onChunk and return
//
// This mirrors the loop that ollama.genClient.ChatWithTools performs, but uses
// the API-native tool-calling format instead of <tool_call> XML tags.

func (c *Client) ChatWithTools(
	ctx context.Context,
	msgs []ollama.ChatMessageWithTools,
	tools []ollama.ToolDef,
	exec ollama.ToolExecutor,
	onChunk func(string),
) (string, ollama.TokenUsage, error) {
	// Build initial history from incoming messages.
	history := make([]openAPIMessage, len(msgs))
	for i, m := range msgs {
		history[i] = openAPIMessage{Role: m.Role, Content: m.Content}
	}

	// Convert ToolDef slice to OpenAI-compatible tools array.
	apiTools := toOpenAPITools(tools)

	var totalUsage ollama.TokenUsage
	const maxIterations = 60

	for iter := 0; iter < maxIterations; iter++ {
		emit(onChunk, "🤔 thinking… (turn %d/%d)", iter+1, maxIterations)

		req := openAPIChatRequest{
			Model:      c.model,
			Stream:     false,
			Messages:   history,
			Tools:      apiTools,
			ToolChoice: "auto",
		}

		body, err := json.Marshal(req)
		if err != nil {
			return "", totalUsage, fmt.Errorf("openapi: marshal request: %w", err)
		}

		reqCtx, cancel := context.WithTimeout(ctx, apiTimeout)
		httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			cancel()
			return "", totalUsage, fmt.Errorf("openapi: create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.http.Do(httpReq)
		if err != nil {
			cancel()
			return "", totalUsage, fmt.Errorf("openapi: http error: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			resp.Body.Close()
			cancel()
			return "", totalUsage, fmt.Errorf("openapi: %d: %s", resp.StatusCode, string(b))
		}

		var out openAPINonStreamResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			resp.Body.Close()
			cancel()
			return "", totalUsage, fmt.Errorf("openapi: decode response: %w", err)
		}
		resp.Body.Close()
		cancel()

		if out.Error != nil && out.Error.Message != "" {
			return "", totalUsage, fmt.Errorf("openapi: %s", out.Error.Message)
		}

		if out.Usage != nil {
			totalUsage.PromptTokens += out.Usage.PromptTokens
			totalUsage.CompletionTokens += out.Usage.CompletionTokens
		}

		if len(out.Choices) == 0 {
			return "", totalUsage, fmt.Errorf("openapi: empty choices")
		}

		choice := out.Choices[0]

		// If the model wants to call tools, execute each and loop.
		if len(choice.Message.ToolCalls) > 0 {
			// Add the assistant message with tool_calls to history.
			history = append(history, choice.Message)

			for _, tc := range choice.Message.ToolCalls {
				toolName := tc.Function.Name
				var args map[string]any
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
					args = map[string]any{}
				}
				emit(onChunk, "🔧 %s(%s)", toolName, summarizeArgs(args))

				result := ""
				if exec != nil {
					result = exec(toolName, args)
				}
				if result == "" {
					result = "ok"
				}

				// Truncate very long results to keep context manageable.
				const maxResultLen = 2000
				if len(result) > maxResultLen {
					result = result[:maxResultLen] + "..."
				}

				history = append(history, openAPIMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    result,
				})
				emit(onChunk, " ✓ (%d bytes)", len(result))
			}
			emit(onChunk, "\n")
			continue
		}

		// No tool calls — this is the final text response.
		text := choice.Message.Content
		if onChunk != nil && text != "" {
			onChunk(text)
		}
		return text, totalUsage, nil
	}

	return "", totalUsage, fmt.Errorf("openapi: tool-calling loop exceeded %d iterations", maxIterations)
}

// -- helpers ------------------------------------------------------------------

// emit sends a formatted status line to onChunk (newline-terminated).
func emit(onChunk func(string), format string, args ...any) {
	if onChunk == nil {
		return
	}
	onChunk(fmt.Sprintf(format+"\n", args...))
}

// summarizeArgs produces a human-readable summary of tool arguments for logging.
func summarizeArgs(args map[string]any) string {
	if path, ok := args["path"]; ok {
		return fmt.Sprintf("path=%s", path)
	}
	// Fallback: key=value pairs, omitting "content" to avoid flooding log.
	parts := make([]string, 0, len(args))
	for k, v := range args {
		if k == "content" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ", ")
}

// toOpenAPIMessages converts ollama.ChatMessage slice to openAPIMessage slice.
func toOpenAPIMessages(msgs []ollama.ChatMessage) []openAPIMessage {
	out := make([]openAPIMessage, len(msgs))
	for i, m := range msgs {
		out[i] = openAPIMessage{Role: m.Role, Content: m.Content}
	}
	return out
}

// toOpenAPITools converts ollama.ToolDef slice to the OpenAI-compatible format.
func toOpenAPITools(tools []ollama.ToolDef) []openAPITool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]openAPITool, len(tools))
	for i, t := range tools {
		out[i] = openAPITool{
			Type: "function",
			Function: openAPIFunction{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		}
	}
	return out
}
