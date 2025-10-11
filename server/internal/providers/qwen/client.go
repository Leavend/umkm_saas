package qwen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"server/internal/infra"
)

// ErrMissingAPIKey indicates that the client was configured without credentials.
var ErrMissingAPIKey = errors.New("qwen: api key is required")

// Options configures the DashScope Qwen client.
type Options struct {
	APIKey         string
	BaseURL        string
	Model          string
	DefaultSize    string
	PromptExtend   bool
	Watermark      bool
	HTTPClient     *http.Client
	Logger         *infra.Logger
	RequestTimeout time.Duration
}

// Client performs HTTP calls to the DashScope Qwen text-to-image API.
type Client struct {
	apiKey       string
	baseURL      string
	model        string
	defaultSize  string
	promptExtend bool
	watermark    bool
	httpClient   *http.Client
	logger       *infra.Logger
}

// ImageRequest captures the required inputs for image generation.
type ImageRequest struct {
	Prompt         string
	NegativePrompt string
	Size           string
	Seed           int
	RequestID      string
	Quality        string
	Locale         string
	Workflow       Workflow
	SourceImage    *SourceImage
}

// ImageAsset is the normalized result from the Qwen API.
type ImageAsset struct {
	URL    string
	Data   []byte
	Format string
	Width  int
	Height int
}

// Workflow describes editing directives when conditioning on an input image.
type Workflow struct {
	Mode            string
	BackgroundTheme string
	BackgroundStyle string
	EnhanceLevel    string
	RetouchStrength string
	Notes           string
}

// SourceImage represents an uploaded asset to be edited by the Qwen model.
type SourceImage struct {
	AssetID    string
	StorageKey string
	URL        string
	MIME       string
	Filename   string
	Data       []byte `json:"-"`
	Width      int
	Height     int
}

type generationRequest struct {
	Model      string           `json:"model"`
	Input      generationInput  `json:"input"`
	Parameters generationParams `json:"parameters"`
}

type generationInput struct {
	Messages []generationMessage `json:"messages"`
}

type generationMessage struct {
	Role    string              `json:"role"`
	Content []generationContent `json:"content"`
}

type generationContent struct {
	Text  string           `json:"text,omitempty"`
	Image *generationImage `json:"image,omitempty"`
}

type generationImage struct {
	Format string `json:"format,omitempty"`
	Data   string `json:"image_bytes,omitempty"`
	URL    string `json:"url,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Name   string `json:"name,omitempty"`
}

type generationParams struct {
	NegativePrompt string          `json:"negative_prompt,omitempty"`
	Size           string          `json:"size,omitempty"`
	PromptExtend   *bool           `json:"prompt_extend,omitempty"`
	Watermark      *bool           `json:"watermark,omitempty"`
	Seed           *int            `json:"seed,omitempty"`
	Quality        string          `json:"quality,omitempty"`
	Style          string          `json:"style,omitempty"`
	Locale         string          `json:"locale,omitempty"`
	Workflow       *workflowParams `json:"workflow,omitempty"`
}

type workflowParams struct {
	Mode            string `json:"mode,omitempty"`
	BackgroundTheme string `json:"background_theme,omitempty"`
	BackgroundStyle string `json:"background_style,omitempty"`
	EnhanceLevel    string `json:"enhance_level,omitempty"`
	RetouchStrength string `json:"retouch_strength,omitempty"`
	Notes           string `json:"notes,omitempty"`
}

type generationResponse struct {
	Output struct {
		Choices []struct {
			Message struct {
				Content []struct {
					Image string `json:"image"`
				} `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	} `json:"output"`
	Usage struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewClient constructs a client with sane defaults and injected dependencies.
func NewClient(opts Options) (*Client, error) {
	httpClient := opts.HTTPClient
	if httpClient == nil {
		timeout := opts.RequestTimeout
		if timeout <= 0 {
			timeout = 45 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}
	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://dashscope-intl.aliyuncs.com/api/v1"
	}
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = "qwen-image-plus"
	}
	defaultSize := strings.TrimSpace(opts.DefaultSize)
	if defaultSize == "" {
		defaultSize = "1328*1328"
	}
	var logger *infra.Logger
	if opts.Logger != nil {
		logger = opts.Logger
	} else {
		discard := zerolog.New(io.Discard)
		l := infra.Logger(discard)
		logger = &l
	}
	return &Client{
		apiKey:       strings.TrimSpace(opts.APIKey),
		baseURL:      baseURL,
		model:        model,
		defaultSize:  defaultSize,
		promptExtend: opts.PromptExtend,
		watermark:    opts.Watermark,
		httpClient:   httpClient,
		logger:       logger,
	}, nil
}

// Model returns the configured model identifier.
func (c *Client) Model() string {
	return c.model
}

// HasCredentials reports whether the client can perform remote calls.
func (c *Client) HasCredentials() bool {
	return c.apiKey != ""
}

// GenerateImage invokes the DashScope API once and returns a single image asset.
func (c *Client) GenerateImage(ctx context.Context, req ImageRequest) (*ImageAsset, error) {
	if !c.HasCredentials() {
		return nil, ErrMissingAPIKey
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, errors.New("qwen: prompt is required")
	}
	contents := make([]generationContent, 0, 2)
	if img := encodeImageContent(req.SourceImage); img != nil {
		contents = append(contents, generationContent{Image: img})
	}
	contents = append(contents, generationContent{Text: prompt})
	payload := generationRequest{
		Model: c.model,
		Input: generationInput{
			Messages: []generationMessage{{
				Role:    "user",
				Content: contents,
			}},
		},
		Parameters: generationParams{},
	}
	if neg := strings.TrimSpace(req.NegativePrompt); neg != "" {
		payload.Parameters.NegativePrompt = neg
	}
	size := strings.TrimSpace(req.Size)
	if size == "" {
		size = c.defaultSize
	}
	payload.Parameters.Size = size
	if quality := strings.TrimSpace(req.Quality); quality != "" {
		payload.Parameters.Quality = quality
	}
	payload.Parameters.Style = "product-photography"
	if extend := c.promptExtend; extend {
		payload.Parameters.PromptExtend = &extend
	}
	if req.Seed > 0 {
		payload.Parameters.Seed = &req.Seed
	}
	watermark := c.watermark
	payload.Parameters.Watermark = &watermark
	if loc := strings.TrimSpace(req.Locale); loc != "" {
		payload.Parameters.Locale = loc
	}
	if wf := buildWorkflowParams(req.Workflow); wf != nil {
		payload.Parameters.Workflow = wf
	}

	endpoint := c.baseURL + "/services/aigc/multimodal-generation/generation"
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("qwen: encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("qwen: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("qwen: http request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("qwen: read response: %w", err)
	}

	if resp.StatusCode >= 300 {
		var detail errorResponse
		if err := json.Unmarshal(raw, &detail); err == nil && detail.Message != "" {
			return nil, fmt.Errorf("qwen: %s (%s)", detail.Message, detail.Code)
		}
		return nil, fmt.Errorf("qwen: status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var decoded generationResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("qwen: decode response: %w", err)
	}
	if decoded.Code != "" {
		return nil, fmt.Errorf("qwen: %s (%s)", decoded.Message, decoded.Code)
	}
	imageURL := firstImageURL(decoded)
	if imageURL == "" {
		return nil, errors.New("qwen: empty image url")
	}
	data, format, err := c.download(ctx, imageURL)
	if err != nil {
		return nil, err
	}
	width, height := decoded.Usage.Width, decoded.Usage.Height
	if width == 0 || height == 0 {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err == nil {
			width, height = cfg.Width, cfg.Height
		}
	}
	c.logger.Debug().
		Str("model", c.model).
		Str("request_id", decoded.RequestID).
		Str("url", imageURL).
		Msg("qwen: generated image asset")
	return &ImageAsset{URL: imageURL, Data: data, Format: format, Width: width, Height: height}, nil
}

func (c *Client) download(ctx context.Context, imageURL string) ([]byte, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(imageURL))
	if err != nil || parsed.Scheme == "" {
		return nil, "", fmt.Errorf("qwen: invalid image url: %s", imageURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("qwen: build download request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("qwen: download image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("qwen: download status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("qwen: read image: %w", err)
	}
	format := resp.Header.Get("Content-Type")
	if format == "" {
		format = "image/png"
	}
	return data, format, nil
}

func firstImageURL(resp generationResponse) string {
	for _, choice := range resp.Output.Choices {
		for _, content := range choice.Message.Content {
			if url := strings.TrimSpace(content.Image); url != "" {
				return url
			}
		}
	}
	return ""
}

func encodeImageContent(src *SourceImage) *generationImage {
	if src == nil {
		return nil
	}
	if len(src.Data) == 0 && strings.TrimSpace(src.URL) == "" {
		return nil
	}
	payload := &generationImage{}
	if strings.TrimSpace(src.MIME) != "" {
		payload.Format = strings.TrimPrefix(strings.ToLower(src.MIME), "image/")
	}
	if len(src.Data) > 0 {
		payload.Data = base64.StdEncoding.EncodeToString(src.Data)
		payload.Width = src.Width
		payload.Height = src.Height
	}
	if payload.Format == "" && strings.TrimSpace(src.Filename) != "" {
		if idx := strings.LastIndex(src.Filename, "."); idx > -1 && idx < len(src.Filename)-1 {
			payload.Format = strings.TrimPrefix(strings.ToLower(src.Filename[idx+1:]), ".")
		}
	}
	if strings.TrimSpace(src.URL) != "" {
		payload.URL = strings.TrimSpace(src.URL)
	}
	if strings.TrimSpace(src.Filename) != "" {
		payload.Name = strings.TrimSpace(src.Filename)
	}
	return payload
}

func buildWorkflowParams(cfg Workflow) *workflowParams {
	mode := strings.TrimSpace(cfg.Mode)
	if mode == "" || mode == "generate" {
		return nil
	}
	params := &workflowParams{Mode: mode}
	if cfg.BackgroundTheme != "" {
		params.BackgroundTheme = strings.TrimSpace(cfg.BackgroundTheme)
	}
	if cfg.BackgroundStyle != "" {
		params.BackgroundStyle = strings.TrimSpace(cfg.BackgroundStyle)
	}
	if cfg.EnhanceLevel != "" {
		params.EnhanceLevel = strings.TrimSpace(cfg.EnhanceLevel)
	}
	if cfg.RetouchStrength != "" {
		params.RetouchStrength = strings.TrimSpace(cfg.RetouchStrength)
	}
	if cfg.Notes != "" {
		params.Notes = strings.TrimSpace(cfg.Notes)
	}
	return params
}
