package qwen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEncodeImageContentWithInlineData(t *testing.T) {
	src := &SourceImage{
		Data:     []byte{0xde, 0xad, 0xbe, 0xef},
		Width:    640,
		Height:   480,
		MIME:     "image/png",
		Filename: "ignored.jpg",
		URL:      "https://cdn.example.com/some.png",
	}

	encoded := encodeImageContent(src)
	if encoded == nil {
		t.Fatalf("expected encoded payload")
	}
	if encoded.Format != "png" {
		t.Fatalf("format = %q, want png", encoded.Format)
	}
	if encoded.MIMEType != "image/png" {
		t.Fatalf("mime_type = %q, want image/png", encoded.MIMEType)
	}
	if encoded.Width != src.Width || encoded.Height != src.Height {
		t.Fatalf("dimensions = %dx%d, want %dx%d", encoded.Width, encoded.Height, src.Width, src.Height)
	}
	if encoded.Data == "" {
		t.Fatalf("expected data to be populated")
	}
	if encoded.URL != "" {
		t.Fatalf("url should be empty when data is provided")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded.Data)
	if err != nil {
		t.Fatalf("data not base64 encoded: %v", err)
	}
	if !bytes.Equal(decoded, src.Data) {
		t.Fatalf("decoded bytes mismatch: %v vs %v", decoded, src.Data)
	}
}

func TestEncodeImageContentWithURLMetadata(t *testing.T) {
	src := &SourceImage{
		AssetID: "asset-123",
		URL:     "https://cdn.example.com/assets/product.SHOT.JPEG?signature=token",
	}

	encoded := encodeImageContent(src)
	if encoded == nil {
		t.Fatalf("expected encoded payload")
	}
	if encoded.URL != strings.TrimSpace(src.URL) {
		t.Fatalf("url = %q, want %q", encoded.URL, src.URL)
	}
	if encoded.Format != "jpeg" {
		t.Fatalf("format = %q, want jpeg", encoded.Format)
	}
	if encoded.Name != src.AssetID {
		t.Fatalf("name = %q, want %q", encoded.Name, src.AssetID)
	}
}

func TestGenerateImageEditingPayload(t *testing.T) {
	transport := &captureTransport{responses: map[string]responseStub{}}
	client, err := NewClient(Options{
		APIKey:       "test",
		Model:        "qwen-image-plus",
		PromptExtend: true,
		Watermark:    true,
		HTTPClient:   &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
        transport.setJSONResponse("/api/v1/services/aigc/multimodal-generation/generation", map[string]any{
		"output": map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"content": []any{
							map[string]any{"image": "https://example.com/generated/out.png"},
						},
					},
				},
			},
		},
		"usage":      map[string]any{"width": 1024, "height": 1024},
		"request_id": "req-123",
	})
	transport.setBinaryResponse("https://example.com/generated/out.png", []byte{0x89, 'P', 'N', 'G'})

	asset, err := client.GenerateImage(context.Background(), ImageRequest{
		Prompt:      "edit the image",
		Size:        "1024*1024",
		Quality:     "hd",
		SourceImage: &SourceImage{Data: []byte{0x01, 0x02, 0x03}, MIME: "image/png"},
	})
	if err != nil {
		t.Fatalf("generate image: %v", err)
	}
	if asset == nil {
		t.Fatalf("expected asset from generate image")
	}
	if len(asset.Data) == 0 {
		t.Fatalf("expected downloaded image data")
	}
	if transport.lastBody == nil {
		t.Fatalf("expected payload to be captured")
	}

	var payload map[string]any
	if err := json.Unmarshal(transport.lastBody, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	params := payload["parameters"].(map[string]any)
	if _, ok := params["style"]; ok {
		t.Fatalf("style parameter should be omitted for editing")
	}
	if _, ok := params["prompt_extend"]; ok {
		t.Fatalf("prompt_extend should be omitted for editing")
	}
	workflow, ok := params["workflow"].(map[string]any)
	if !ok {
		t.Fatalf("workflow parameters missing")
	}
	if mode := workflow["mode"]; mode != "enhance" {
		t.Fatalf("workflow.mode = %v, want enhance", mode)
	}

	input := payload["input"].(map[string]any)
	messages := input["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(messages))
	}
	content := messages[0].(map[string]any)["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("content len = %d, want 2", len(content))
	}
	if text := content[0].(map[string]any)["text"]; text != "edit the image" {
		t.Fatalf("first content text = %v, want %s", text, "edit the image")
	}
	imageNode := content[1].(map[string]any)["image"].(map[string]any)
	if _, ok := imageNode["image_url"]; ok {
		t.Fatalf("image_url should be omitted for inline edits")
	}
	if _, ok := imageNode["image_bytes"]; !ok {
		t.Fatalf("image_bytes missing from payload")
	}
	if mime := imageNode["mime_type"]; mime != "image/png" {
		t.Fatalf("mime_type = %v, want image/png", mime)
	}
}

type captureTransport struct {
	responses map[string]responseStub
	lastBody  []byte
}

type responseStub struct {
	status int
	header http.Header
	body   []byte
}

func (c *captureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodPost {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body.Close()
		c.lastBody = body
		if stub, ok := c.responses[req.URL.Path]; ok {
			return stub.toResponse(), nil
		}
	}
	if req.Method == http.MethodGet {
		if stub, ok := c.responses[req.URL.String()]; ok {
			return stub.toResponse(), nil
		}
	}
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("not found")),
	}, nil
}

func (c *captureTransport) setJSONResponse(path string, payload any) {
	body, _ := json.Marshal(payload)
	c.responses[path] = responseStub{
		status: http.StatusOK,
		header: http.Header{"Content-Type": []string{"application/json"}},
		body:   body,
	}
}

func (c *captureTransport) setBinaryResponse(url string, data []byte) {
	c.responses[url] = responseStub{
		status: http.StatusOK,
		header: http.Header{"Content-Type": []string{"image/png"}},
		body:   data,
	}
}

func (s responseStub) toResponse() *http.Response {
	header := http.Header{}
	for k, values := range s.header {
		cloned := make([]string, len(values))
		copy(cloned, values)
		header[k] = cloned
	}
	return &http.Response{
		StatusCode: s.status,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(s.body)),
	}
}
