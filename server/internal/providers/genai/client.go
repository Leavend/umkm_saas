package genai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"server/internal/infra"
)

// Options controls how the Gemini client is configured.
type Options struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
	Logger     *infra.Logger
}

// Client provides a lightweight facade over Gemini so that providers can focus
// on translating domain requests to API calls. The real HTTP invocation is
// intentionally stubbed with deterministic synthetic assets until the external
// integration is wired. This keeps the worker fully operational in local and CI
// environments while preserving the extension points for real API calls.
type Client struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	logger     *infra.Logger
}

// ImageRequest represents the information required to generate images.
type ImageRequest struct {
	Prompt       string
	Quantity     int
	AspectRatio  string
	Locale       string
	WatermarkTag string
	RequestID    string
}

// VideoRequest represents the information required to generate a video.
type VideoRequest struct {
	Prompt    string
	Locale    string
	RequestID string
}

// ImageAsset is the normalized representation returned by the Gemini client.
type ImageAsset struct {
	StorageKey string
	URL        string
	Format     string
	Width      int
	Height     int
	Data       []byte
}

// VideoAsset is the normalized representation of a generated video.
type VideoAsset struct {
	StorageKey string
	URL        string
	Format     string
	Length     int
	Data       []byte
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts,omitempty"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
	FileData   *geminiFileData   `json:"fileData,omitempty"`
}

type geminiTool struct {
	ImageGeneration *geminiImageTool `json:"image_generation,omitempty"`
	VideoGeneration *geminiVideoTool `json:"video_generation,omitempty"`
}

type geminiImageTool struct{}

type geminiVideoTool struct{}

type geminiInlineData struct {
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
}

type geminiFileData struct {
	MimeType string `json:"mimeType,omitempty"`
	FileURI  string `json:"fileUri,omitempty"`
}

type geminiGenerationConfig struct {
	CandidateCount   int    `json:"candidateCount,omitempty"`
	ResponseMimeType string `json:"responseMimeType,omitempty"`
}

type geminiGenerateContentRequest struct {
	Contents         []geminiContent         `json:"contents"`
	Tools            []geminiTool            `json:"tools,omitempty"`
	ToolConfig       *geminiToolConfig       `json:"tool_config,omitempty"`
	GenerationConfig *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiToolConfig struct {
	ImageGenerationConfig *geminiImageGenerationConfig `json:"image_generation_config,omitempty"`
	VideoGenerationConfig *geminiVideoGenerationConfig `json:"video_generation_config,omitempty"`
}

type geminiImageGenerationConfig struct {
	NumberOfImages int `json:"number_of_images,omitempty"`
}

type geminiVideoGenerationConfig struct{}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason,omitempty"`
}

type geminiGenerateContentResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiErrorResponse struct {
	Error struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"error"`
}

// NewClient constructs a Gemini client with sane defaults. Callers may provide
// a nil HTTP client; a reusable one with sensible timeouts will be created.
func NewClient(opts Options) (*Client, error) {
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}

	model := opts.Model
	if model == "" {
		model = "gemini-2.5-flash"
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
		apiKey:     strings.TrimSpace(opts.APIKey),
		baseURL:    baseURL,
		model:      model,
		httpClient: client,
		logger:     logger,
	}, nil
}

// Model returns the configured Gemini model identifier.
func (c *Client) Model() string {
	return c.model
}

// GenerateImages synthesizes deterministic image assets. In production this is
// where the Gemini image API should be called. The deterministic placeholder
// keeps the rest of the pipeline (DB persistence, asset metadata, etc.)
// exercised so the worker can be verified end-to-end.
func (c *Client) GenerateImages(ctx context.Context, req ImageRequest) ([]ImageAsset, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if c.apiKey == "" {
		return c.syntheticImages(req)
	}

	assets, err := c.remoteGenerateImages(ctx, req)
	if err != nil {
		c.logger.Warn().
			Err(err).
			Str("model", c.model).
			Msg("genai: remote image generation failed; falling back to synthetic assets")
		return c.syntheticImages(req)
	}
	if len(assets) == 0 {
		return c.syntheticImages(req)
	}
	return assets, nil
}

// GenerateVideo synthesizes a deterministic video asset placeholder.
func (c *Client) GenerateVideo(ctx context.Context, req VideoRequest) (*VideoAsset, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if c.apiKey == "" {
		return c.syntheticVideo(req), nil
	}

	asset, err := c.remoteGenerateVideo(ctx, req)
	if err != nil {
		c.logger.Warn().
			Err(err).
			Str("model", c.model).
			Msg("genai: remote video generation failed; falling back to synthetic asset")
		return c.syntheticVideo(req), nil
	}
	if asset == nil || len(asset.Data) == 0 {
		return c.syntheticVideo(req), nil
	}
	return asset, nil
}

func (c *Client) syntheticImages(req ImageRequest) ([]ImageAsset, error) {
	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 1
	}

	width, height := normalizeAspect(req.AspectRatio)
	assets := make([]ImageAsset, quantity)
	for i := 0; i < quantity; i++ {
		seed := deterministicSeed(req.RequestID, req.Prompt, req.Locale, req.WatermarkTag, i)
		storageKey := syntheticStorageKey("image", c.model, seed, i+1, "png")
		img := renderSyntheticImage(width, height, seed, req.Prompt)
		assets[i] = ImageAsset{
			StorageKey: storageKey,
			URL:        c.assetURL(storageKey),
			Format:     "image/png",
			Width:      width,
			Height:     height,
			Data:       img,
		}
	}

	c.logger.Debug().
		Str("request_id", req.RequestID).
		Str("model", c.model).
		Int("quantity", quantity).
		Msg("genai: generated synthetic image assets")

	return assets, nil
}

func (c *Client) syntheticVideo(req VideoRequest) *VideoAsset {
	seed := deterministicSeed(req.RequestID, req.Prompt, req.Locale, c.model, 0)
	storageKey := syntheticStorageKey("video", c.model, seed, 1, "mp4")
	asset := &VideoAsset{
		StorageKey: storageKey,
		URL:        c.assetURL(storageKey),
		Format:     "video/mp4",
		Length:     estimateVideoLength(req.Prompt),
		Data:       renderSyntheticVideo(seed, req.Prompt),
	}

	c.logger.Debug().
		Str("request_id", req.RequestID).
		Str("model", c.model).
		Msg("genai: generated synthetic video asset")

	return asset
}

func (c *Client) remoteGenerateImages(ctx context.Context, req ImageRequest) ([]ImageAsset, error) {
	quantity := clampQuantity(req.Quantity)
	payload := geminiGenerateContentRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{{
					Text: buildImagePrompt(req),
				}},
			},
		},
		Tools: []geminiTool{{ImageGeneration: &geminiImageTool{}}},
		ToolConfig: &geminiToolConfig{
			ImageGenerationConfig: &geminiImageGenerationConfig{
				NumberOfImages: quantity,
			},
		},
	}

	var response geminiGenerateContentResponse
	if err := c.invokeGemini(ctx, fmt.Sprintf("/models/%s:generateContent", url.PathEscape(c.model)), payload, &response); err != nil {
		return nil, err
	}

	width, height := normalizeAspect(req.AspectRatio)
	var assets []ImageAsset
	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			asset, err := c.decodeInlineAsset(ctx, part)
			if err != nil || len(asset.Data) == 0 {
				continue
			}
			format := asset.Format
			if format == "" {
				format = "image/png"
			}
			w, h := decodeImageDimensions(asset.Data)
			if w == 0 || h == 0 {
				w, h = width, height
			}
			assets = append(assets, ImageAsset{
				StorageKey: "",
				URL:        asset.URL,
				Format:     format,
				Width:      w,
				Height:     h,
				Data:       asset.Data,
			})
			if len(assets) >= quantity {
				break
			}
		}
		if len(assets) >= quantity {
			break
		}
	}

	c.logger.Debug().
		Str("request_id", req.RequestID).
		Str("model", c.model).
		Int("quantity", len(assets)).
		Msg("genai: generated remote image assets")

	return assets, nil
}

func (c *Client) remoteGenerateVideo(ctx context.Context, req VideoRequest) (*VideoAsset, error) {
	payload := geminiGenerateContentRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{{
					Text: buildVideoPrompt(req),
				}},
			},
		},
		Tools: []geminiTool{{VideoGeneration: &geminiVideoTool{}}},
	}

	var response geminiGenerateContentResponse
	if err := c.invokeGemini(ctx, fmt.Sprintf("/models/%s:generateContent", url.PathEscape(c.model)), payload, &response); err != nil {
		return nil, err
	}

	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			asset, err := c.decodeInlineAsset(ctx, part)
			if err != nil || len(asset.Data) == 0 {
				continue
			}
			length := estimateVideoLength(req.Prompt)
			if asset.Length > 0 {
				length = asset.Length
			}
			result := &VideoAsset{
				StorageKey: "",
				URL:        asset.URL,
				Format:     asset.Format,
				Length:     length,
				Data:       asset.Data,
			}
			c.logger.Debug().
				Str("request_id", req.RequestID).
				Str("model", c.model).
				Msg("genai: generated remote video asset")

			return result, nil
		}
	}

	return nil, fmt.Errorf("no video content returned")
}

type inlineAsset struct {
	Data   []byte
	Format string
	URL    string
	Length int
}

func (c *Client) invokeGemini(ctx context.Context, path string, payload any, out any) error {
	endpoint := strings.TrimRight(c.baseURL, "/") + path
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	q := req.URL.Query()
	if c.apiKey != "" {
		q.Set("key", c.apiKey)
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("invoke gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		var apiErr geminiErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error.Message != "" {
			return fmt.Errorf("gemini status %d: %s", resp.StatusCode, apiErr.Error.Message)
		}
		data, _ := io.ReadAll(resp.Body)
		if len(data) > 0 {
			return fmt.Errorf("gemini status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
		}
		return fmt.Errorf("gemini status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode gemini response: %w", err)
	}
	return nil
}

func (c *Client) decodeInlineAsset(ctx context.Context, part geminiPart) (inlineAsset, error) {
	if part.InlineData != nil && part.InlineData.Data != "" {
		data, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
		if err != nil {
			return inlineAsset{}, fmt.Errorf("decode inline data: %w", err)
		}
		return inlineAsset{Data: data, Format: part.InlineData.MimeType}, nil
	}

	if part.FileData != nil && part.FileData.FileURI != "" {
		data, mime, err := c.downloadFile(ctx, part.FileData.FileURI)
		if err != nil {
			return inlineAsset{}, err
		}
		return inlineAsset{Data: data, Format: firstNonEmpty(part.FileData.MimeType, mime), URL: part.FileData.FileURI}, nil
	}

	return inlineAsset{}, nil
}

func (c *Client) downloadFile(ctx context.Context, uri string) ([]byte, string, error) {
	target := uri
	if !strings.HasPrefix(uri, "http://") && !strings.HasPrefix(uri, "https://") {
		target = strings.TrimRight(c.baseURL, "/") + "/" + strings.TrimLeft(uri, "/")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create download request: %w", err)
	}
	if c.apiKey != "" {
		q := req.URL.Query()
		q.Set("key", c.apiKey)
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		data, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("download file status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	blob, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read file: %w", err)
	}
	return blob, resp.Header.Get("Content-Type"), nil
}

func buildImagePrompt(req ImageRequest) string {
	var b strings.Builder
	prompt := strings.TrimSpace(req.Prompt)
	if prompt != "" {
		b.WriteString(prompt)
	}
	if aspect := strings.TrimSpace(req.AspectRatio); aspect != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("Aspect ratio: ")
		b.WriteString(aspect)
	}
	if watermark := strings.TrimSpace(req.WatermarkTag); watermark != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("Watermark tag: ")
		b.WriteString(watermark)
	}
	if locale := strings.TrimSpace(req.Locale); locale != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("Locale: ")
		b.WriteString(locale)
	}
	if b.Len() == 0 {
		b.WriteString("Create a marketing image")
	}
	return b.String()
}

func buildVideoPrompt(req VideoRequest) string {
	var b strings.Builder
	prompt := strings.TrimSpace(req.Prompt)
	if prompt != "" {
		b.WriteString(prompt)
	}
	if locale := strings.TrimSpace(req.Locale); locale != "" {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("Locale: ")
		b.WriteString(locale)
	}
	if b.Len() == 0 {
		b.WriteString("Create a short promotional video")
	}
	return b.String()
}

func clampQuantity(quantity int) int {
	if quantity <= 0 {
		return 1
	}
	if quantity > 4 {
		return 4
	}
	return quantity
}

func decodeImageDimensions(data []byte) (int, int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (c *Client) assetURL(storageKey string) string {
	if storageKey == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", strings.TrimRight(c.baseURL, "/"), strings.TrimLeft(storageKey, "/"))
}

func syntheticStorageKey(kind, model, seed string, index int, ext string) string {
	escapedModel := url.PathEscape(model)
	escapedKind := url.PathEscape(kind)
	return fmt.Sprintf("synthetic/%s/%s-%s/%02d.%s", escapedModel, escapedKind, seed, index, ext)
}

func renderSyntheticImage(width, height int, seed, prompt string) []byte {
	if width <= 0 {
		width = 1024
	}
	if height <= 0 {
		height = 1024
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	base := colorFromSeed(seed, 0)
	accent := colorFromSeed(seed, 1)
	draw.Draw(img, img.Bounds(), &image.Uniform{base}, image.Point{}, draw.Src)

	stripeHeight := maxInt(32, height/12)
	for y := 0; y < height; y += stripeHeight * 2 {
		stripe := image.Rect(0, y, width, minInt(height, y+stripeHeight))
		draw.Draw(img, stripe, &image.Uniform{accent}, image.Point{}, draw.Over)
	}

	diagonal := colorFromSeed(seed, 2)
	for i := 0; i < maxInt(width, height); i += maxInt(16, width/32) {
		x := i
		for y := 0; y < height; y++ {
			xx := x + y
			if xx >= width {
				break
			}
			img.Set(xx, y, diagonal)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func renderSyntheticVideo(seed, prompt string) []byte {
	lines := []string{
		"Synthetic Gemini video placeholder", fmt.Sprintf("Seed: %s", seed), fmt.Sprintf("Prompt: %s", strings.TrimSpace(prompt)), "", "This placeholder represents where rendered video bytes would be stored once the", "Gemini video API integration is enabled."}
	return []byte(strings.Join(lines, "\n"))
}

func colorFromSeed(seed string, shift int) color.RGBA {
	if seed == "" {
		seed = "000000"
	}
	doubled := seed + seed
	start := (shift * 6) % len(seed)
	segment := doubled[start : start+6]
	r := mustParseHexByte(segment[0:2])
	g := mustParseHexByte(segment[2:4])
	b := mustParseHexByte(segment[4:6])
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func mustParseHexByte(s string) uint8 {
	v, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		return 0
	}
	return uint8(v)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func deterministicSeed(parts ...any) string {
	hasher := sha256.New()
	for _, part := range parts {
		hasher.Write([]byte(fmt.Sprintf("%v", part)))
		hasher.Write([]byte{'|'})
	}
	return hex.EncodeToString(hasher.Sum(nil))[:16]
}

func normalizeAspect(aspect string) (int, int) {
	switch strings.TrimSpace(strings.ToLower(aspect)) {
	case "16:9":
		return 1920, 1080
	case "9:16":
		return 1080, 1920
	case "4:5":
		return 1024, 1280
	case "3:2":
		return 1536, 1024
	case "1:1", "square", "":
		return 1024, 1024
	default:
		parts := strings.Split(aspect, ":")
		if len(parts) == 2 {
			if a, errA := strconv.Atoi(strings.TrimSpace(parts[0])); errA == nil {
				if b, errB := strconv.Atoi(strings.TrimSpace(parts[1])); errB == nil && a > 0 && b > 0 {
					width := 1024
					height := int(float64(width) * float64(b) / float64(a))
					return width, height
				}
			}
		}
		return 1024, 1024
	}
}

func estimateVideoLength(prompt string) int {
	words := len(strings.Fields(prompt))
	if words == 0 {
		return 12
	}
	length := words / 3
	if length < 8 {
		return 8
	}
	if length > 45 {
		return 45
	}
	return length
}
