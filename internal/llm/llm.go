package llm

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
)

type Request struct {
	System      string
	User        string
	Model       string
	Temperature float64
	MaxTokens   int

	// Raw inputs, used by the offline (non-AI) generator.
	Diff       string
	Hint       string
	Style      string
	Scopes     []string
	MaxHeader  int
	NoBreaking bool

	// Offline, when set, replaces the offline generator's output (used for
	// replies that aren't commit messages, like pull request descriptions).
	Offline func() string
}

type OnToken func(string)

type Provider interface {
	Generate(ctx context.Context, r Request, on OnToken) (string, error)
}

type Options struct {
	Provider string // preset name or protocol: openai, anthropic, ollama, offline, groq, gemini, ...
	BaseURL  string
	APIKey   string
	Timeout  time.Duration
	NumCtx   int
}

func IsOllama(base string) bool { return strings.Contains(base, ":11434") }

// New builds a provider. Provider may be a preset name (groq, gemini, ...) or a
// protocol (openai, anthropic, ollama, offline); an empty BaseURL means the
// preset's default.
func New(o Options) (Provider, error) {
	client := &http.Client{Timeout: o.Timeout}
	name := o.Provider
	if name == "" {
		name = "openai"
	}
	pre, ok := Lookup(name)
	if !ok {
		return nil, fmt.Errorf("unknown provider %q (choose one of: %s)", o.Provider, strings.Join(Names(), ", "))
	}
	base := strings.TrimRight(o.BaseURL, "/")
	if base == "" {
		base = pre.BaseURL
	}
	switch pre.Kind {
	case KindOffline:
		return Offline{}, nil
	case KindOllama:
		return newOllama(base, o.NumCtx, client), nil
	case KindAnthropic:
		if IsOllama(base) {
			base = pre.BaseURL
		}
		if o.APIKey == "" {
			return nil, &Error{Kind: ErrAuth, Base: base, Detail: "no API key set"}
		}
		return &anthropic{base: base, key: o.APIKey, http: client}, nil
	default:
		if IsOllama(base) {
			return newOllama(base, o.NumCtx, client), nil
		}
		if pre.NeedsKey() && o.APIKey == "" && base == pre.BaseURL {
			return nil, &Error{Kind: ErrAuth, Base: base, Detail: "no API key set"}
		}
		return &openAI{base: base, key: o.APIKey, http: client}, nil
	}
}

type openAI struct {
	base, key string
	http      *http.Client
}

func (p *openAI) Generate(ctx context.Context, r Request, on OnToken) (string, error) {
	payload := map[string]any{
		"model":       r.Model,
		"temperature": r.Temperature,
		"max_tokens":  r.MaxTokens,
		"stream":      true,
		"messages": []map[string]string{
			{"role": "system", "content": r.System},
			{"role": "user", "content": r.User},
		},
	}

	// Reasoning models spend tokens "thinking" before they answer; keep that
	// short and leave room for the actual message.
	if m := strings.ToLower(r.Model); strings.Contains(m, "gpt-oss") || strings.Contains(m, "gemini") {
		payload["reasoning_effort"] = "low"
		if r.MaxTokens < 2048 {
			payload["max_tokens"] = 2048
		}
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", p.base+"/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if p.key != "" {
		req.Header.Set("Authorization", "Bearer "+p.key)
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return "", connError(err, p.base)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", httpError(resp, p.base)
	}

	if !strings.Contains(resp.Header.Get("Content-Type"), "event-stream") {
		var out struct {
			Choices []struct {
				Message struct{ Content string } `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || len(out.Choices) == 0 {
			return "", fmt.Errorf("unexpected response from %s", p.base)
		}
		if on != nil {
			on(out.Choices[0].Message.Content)
		}
		return out.Choices[0].Message.Content, nil
	}

	var full strings.Builder
	err = readSSE(resp.Body, func(data string) bool {
		if data == "[DONE]" {
			return false
		}
		var chunk struct {
			Choices []struct {
				Delta struct{ Content string } `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
			t := chunk.Choices[0].Delta.Content
			full.WriteString(t)
			if on != nil && t != "" {
				on(t)
			}
		}
		return true
	})
	return full.String(), err
}

type ollama struct {
	base   string
	numCtx int
	http   *http.Client
}

func newOllama(base string, numCtx int, c *http.Client) *ollama {
	base = strings.TrimSuffix(strings.TrimRight(base, "/"), "/v1")
	if base == "" {
		base = "http://localhost:11434"
	}
	n := numCtx
	if n <= 0 {
		n = 8192
	}
	return &ollama{base: base, numCtx: n, http: c}
}

func (p *ollama) Generate(ctx context.Context, r Request, on OnToken) (string, error) {
	maxTok := r.MaxTokens
	if maxTok == 0 {
		maxTok = 400
	}
	body, _ := json.Marshal(map[string]any{
		"model":  r.Model,
		"stream": true,
		"options": map[string]any{
			"temperature": r.Temperature,
			"num_ctx":     p.numCtx,
			"num_predict": maxTok,
		},
		"messages": []map[string]string{
			{"role": "system", "content": r.System},
			{"role": "user", "content": r.User},
		},
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", p.base+"/api/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return "", connError(err, p.base)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", httpError(resp, p.base)
	}
	var full strings.Builder
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var chunk struct {
			Message struct{ Content string } `json:"message"`
			Done    bool                     `json:"done"`
			Error   string                   `json:"error"`
		}
		if json.Unmarshal(sc.Bytes(), &chunk) != nil {
			continue
		}
		if chunk.Error != "" {
			return "", fmt.Errorf("ollama: %s", chunk.Error)
		}
		full.WriteString(chunk.Message.Content)
		if on != nil && chunk.Message.Content != "" {
			on(chunk.Message.Content)
		}
		if chunk.Done {
			break
		}
	}
	return full.String(), sc.Err()
}

type anthropic struct {
	base, key string
	http      *http.Client
}

func (p *anthropic) Generate(ctx context.Context, r Request, on OnToken) (string, error) {
	maxTok := r.MaxTokens
	if maxTok == 0 {
		maxTok = 400
	}
	body, _ := json.Marshal(map[string]any{
		"model":       r.Model,
		"max_tokens":  maxTok,
		"temperature": r.Temperature,
		"stream":      true,
		"system":      r.System,
		"messages":    []map[string]string{{"role": "user", "content": r.User}},
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", p.base+"/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.key)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := p.http.Do(req)
	if err != nil {
		return "", connError(err, p.base)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", httpError(resp, p.base)
	}
	var full strings.Builder
	var streamErr error
	err = readSSE(resp.Body, func(data string) bool {
		var ev struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
			Error struct{ Message string } `json:"error"`
		}
		if json.Unmarshal([]byte(data), &ev) != nil {
			return true
		}
		switch ev.Type {
		case "content_block_delta":
			full.WriteString(ev.Delta.Text)
			if on != nil && ev.Delta.Text != "" {
				on(ev.Delta.Text)
			}
		case "message_stop":
			return false
		case "error":
			streamErr = fmt.Errorf("anthropic: %s", ev.Error.Message)
			return false
		}
		return true
	})
	if streamErr != nil {
		return "", streamErr
	}
	return full.String(), err
}

func readSSE(r io.Reader, fn func(string) bool) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		if !fn(strings.TrimSpace(strings.TrimPrefix(line, "data:"))) {
			return nil
		}
	}
	return sc.Err()
}
