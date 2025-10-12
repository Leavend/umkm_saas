package qwen

import (
	"bytes"
	"encoding/base64"
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
