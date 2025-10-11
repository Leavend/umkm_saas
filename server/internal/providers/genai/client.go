package genai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// GenerateVideo synthesizes a deterministic video asset placeholder.
func (c *Client) GenerateVideo(ctx context.Context, req VideoRequest) (*VideoAsset, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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

	return asset, nil
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
