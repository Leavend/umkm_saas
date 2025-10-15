package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
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
	Model  string         `json:"model"`
	Input  qwenInput      `json:"input"`
	Params qwenParameters `json:"parameters"`
}

type qwenInput struct {
	Messages []qwenMessage `json:"messages"`
}

type qwenMessage struct {
	Role    string        `json:"role"`
	Content []qwenContent `json:"content"`
}

type qwenContent struct {
	Text  string     `json:"text,omitempty"`
	Image *qwenImage `json:"image,omitempty"`
}

type qwenImage struct {
	URL      string `json:"image_url,omitempty"`
	Data     string `json:"image_bytes,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	Format   string `json:"format,omitempty"`
	Name     string `json:"name,omitempty"`
}

type qwenParameters struct {
	NegativePrompt string `json:"negative_prompt,omitempty"`
	Watermark      bool   `json:"watermark"`
	Seed           *int   `json:"seed,omitempty"`
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

func (c *QwenClient) EditOnce(ctx context.Context, source SourceImage, instruction string, watermark bool, negative string, seed *int) (string, error) {
	if c == nil {
		return "", errors.New("qwen client not configured")
	}
	if c.token == "" {
		return "", errors.New("qwen: API key is missing")
	}
	payload := qwenRequest{Model: "qwen-image-edit"}
	imageContent, err := buildImageContent(source)
	if err != nil {
		return "", err
	}
	msg := qwenMessage{Role: "user"}
	if imageContent != nil {
		msg.Content = append(msg.Content, qwenContent{Image: imageContent})
	}
	msg.Content = append(msg.Content, qwenContent{Text: instruction})
	payload.Input.Messages = append(payload.Input.Messages, msg)
	payload.Params.Watermark = watermark
	if negative = strings.TrimSpace(negative); negative != "" {
		payload.Params.NegativePrompt = negative
	}
	if seed != nil {
		payload.Params.Seed = seed
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

func buildImageContent(source SourceImage) (*qwenImage, error) {
	hasData := len(source.Data) > 0
	url := strings.TrimSpace(source.URL)
	if !hasData && url == "" {
		return nil, errors.New("qwen: image source required")
	}
	img := &qwenImage{}
	if hasData {
		img.Data = base64.StdEncoding.EncodeToString(source.Data)
	} else {
		img.URL = url
	}
	if name := sanitizeName(source.Name); name != "" {
		img.Name = name
	} else if !hasData {
		if derived := sanitizeName(path.Base(url)); derived != "" {
			img.Name = derived
		}
	}
	format := inferFormat(source.MIMEType, source.Name, url)
	if format == "" {
		format = "png"
	}
	img.Format = format
	mimeType := strings.TrimSpace(source.MIMEType)
	if mimeType == "" {
		mimeType = mimeFromFormat(format)
	}
	img.MIMEType = mimeType
	return img, nil
}

func inferFormat(mimeType, name, url string) string {
	if f := inferFormatFromMIME(mimeType); f != "" {
		return f
	}
	if f := inferFormatFromName(name); f != "" {
		return f
	}
	if f := inferFormatFromName(path.Base(url)); f != "" {
		return f
	}
	return ""
}

func inferFormatFromMIME(mimeType string) string {
	mimeType = strings.TrimSpace(strings.ToLower(mimeType))
	if mimeType == "" {
		return ""
	}
	if strings.HasPrefix(mimeType, "image/") {
		return normalizeFormat(strings.TrimPrefix(mimeType, "image/"))
	}
	return ""
}

func inferFormatFromName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if idx := strings.LastIndex(name, "."); idx > -1 && idx < len(name)-1 {
		ext := name[idx+1:]
		if q := strings.IndexAny(ext, "?#"); q >= 0 {
			ext = ext[:q]
		}
		return normalizeFormat(ext)
	}
	return ""
}

func sanitizeName(name string) string {
	cleaned := strings.TrimSpace(name)
	cleaned = strings.Trim(cleaned, "\t\n\r")
	cleaned = path.Base(cleaned)
	if cleaned == "." || cleaned == "/" {
		return ""
	}
	return cleaned
}

func normalizeFormat(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	ext = strings.TrimPrefix(ext, ".")
	switch ext {
	case "jpeg":
		return "jpeg"
	case "jpg":
		return "jpg"
	case "png":
		return "png"
	case "webp":
		return "webp"
	case "bmp":
		return "bmp"
	case "gif":
		return "gif"
	default:
		return ext
	}
}

func mimeFromFormat(format string) string {
	switch format {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "bmp":
		return "image/bmp"
	case "gif":
		return "image/gif"
	case "":
		return ""
	default:
		return "image/" + format
	}
}
