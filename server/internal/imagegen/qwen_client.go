package imagegen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type QwenOptions struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Timeout    time.Duration
}

type QwenClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

func NewQwenClient(opts QwenOptions) *QwenClient {
	base := strings.TrimRight(opts.BaseURL, "/")
	if base == "" {
		base = "https://dashscope-intl.aliyuncs.com/api/v1"
	}
	client := opts.HTTPClient
	if client == nil {
		timeout := opts.Timeout
		if timeout <= 0 {
			timeout = 60 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &QwenClient{
		httpClient: client,
		baseURL:    base,
		token:      strings.TrimSpace(opts.APIKey),
	}
}

type qwenRequest struct {
	Model string `json:"model"`
	Input struct {
		Messages []struct {
			Role    string        `json:"role"`
			Content []interface{} `json:"content"`
		} `json:"messages"`
	} `json:"input"`
	Parameters struct {
		NegativePrompt string `json:"negative_prompt,omitempty"`
		Watermark      bool   `json:"watermark"`
		Seed           *int   `json:"seed,omitempty"`
	} `json:"parameters"`
}

type qwenResp struct {
	Output struct {
		Choices []struct {
			Message struct {
				Content []map[string]string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *QwenClient) EditOnce(ctx context.Context, imageURL, instruction string, watermark bool, negative string, seed *int) (string, error) {
	if c == nil {
		return "", errors.New("qwen client not configured")
	}
	if c.token == "" {
		return "", errors.New("qwen: API key is missing")
	}
	trimmed := strings.TrimSpace(imageURL)
	if trimmed == "" {
		return "", errors.New("qwen: image url required")
	}
	var payload qwenRequest
	payload.Model = "qwen-image-edit"
	msg := struct {
		Role    string        `json:"role"`
		Content []interface{} `json:"content"`
	}{
		Role: "user",
		Content: []interface{}{
			map[string]string{"image": trimmed},
			map[string]string{"text": instruction},
		},
	}
	payload.Input.Messages = append(payload.Input.Messages, msg)
	payload.Parameters.Watermark = watermark
	if negative = strings.TrimSpace(negative); negative != "" {
		payload.Parameters.NegativePrompt = negative
	}
	if seed != nil {
		payload.Parameters.Seed = seed
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	endpoint := c.baseURL + "/services/aigc/multimodal-generation/generation"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out qwenResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		if resp.StatusCode >= http.StatusBadRequest {
			return "", fmt.Errorf("qwen: http %d", resp.StatusCode)
		}
		return "", err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		if out.Message != "" {
			return "", fmt.Errorf("qwen error: %s (%s)", out.Message, out.Code)
		}
		return "", fmt.Errorf("qwen: http %d", resp.StatusCode)
	}
	if len(out.Output.Choices) == 0 || len(out.Output.Choices[0].Message.Content) == 0 {
		if out.Message != "" {
			return "", fmt.Errorf("qwen error: %s (%s)", out.Message, out.Code)
		}
		return "", errors.New("qwen: empty response")
	}
	url := out.Output.Choices[0].Message.Content[0]["image"]
	if strings.TrimSpace(url) == "" {
		return "", errors.New("qwen: missing image url")
	}
	return url, nil
}
