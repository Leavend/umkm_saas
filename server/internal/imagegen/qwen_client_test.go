package imagegen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQwenClientEditOnce(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}
		var payload qwenRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if payload.Model != "qwen-image-edit" {
			t.Fatalf("unexpected model: %s", payload.Model)
		}
		if len(payload.Input.Messages) != 1 {
			t.Fatalf("unexpected messages length: %d", len(payload.Input.Messages))
		}
		contents := payload.Input.Messages[0].Content
		if len(contents) != 2 {
			t.Fatalf("unexpected content length: %d", len(contents))
		}
		if got := strings.TrimSpace(contents[0].Text); got != "do something" {
			t.Fatalf("instruction mismatch: %s", got)
		}
		if contents[1].Image == nil || contents[1].Image.URL != "https://example.com/in.png" {
			t.Fatalf("image content mismatch: %+v", contents[1].Image)
		}
		resp := qwenResp{}
		resp.Output.Choices = []struct {
			Message struct {
				Content []map[string]string `json:"content"`
			} `json:"message"`
		}{{}}
		resp.Output.Choices[0].Message.Content = []map[string]string{{"image": "https://example.com/out.png"}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewQwenClient(QwenOptions{APIKey: "test-key", BaseURL: ts.URL})
	got, err := client.EditOnce(context.Background(), SourceImage{URL: "https://example.com/in.png"}, "do something", true, "", nil)
	if err != nil {
		t.Fatalf("EditOnce error: %v", err)
	}
	if got != "https://example.com/out.png" {
		t.Fatalf("unexpected url: %s", got)
	}
}

func TestQwenClientMissingKey(t *testing.T) {
	client := NewQwenClient(QwenOptions{})
	if _, err := client.EditOnce(context.Background(), SourceImage{URL: "https://example.com/in.png"}, "instr", false, "", nil); err == nil {
		t.Fatalf("expected error when api key missing")
	}
}

func TestQwenClientUsesBytesPayload(t *testing.T) {
	var captured qwenRequest
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		resp := qwenResp{}
		resp.Output.Choices = []struct {
			Message struct {
				Content []map[string]string `json:"content"`
			} `json:"message"`
		}{{}}
		resp.Output.Choices[0].Message.Content = []map[string]string{{"image": "https://example.com/out.png"}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewQwenClient(QwenOptions{APIKey: "test-key", BaseURL: ts.URL})
	data := []byte{0x89, 0x50, 0x4e, 0x47}
	_, err := client.EditOnce(context.Background(), SourceImage{Data: data, MIMEType: "image/png", Name: "sample.png"}, "instr", false, "", nil)
	if err != nil {
		t.Fatalf("EditOnce error: %v", err)
	}
	if len(captured.Input.Messages) == 0 || len(captured.Input.Messages[0].Content) < 2 {
		t.Fatalf("image bytes not captured: %+v", captured)
	}
	img := captured.Input.Messages[0].Content[1].Image
	if img == nil {
		t.Fatalf("expected image content present")
	}
	if img.Data == "" {
		t.Fatalf("expected base64 data in payload")
	}
	if img.MIMEType != "image/png" {
		t.Fatalf("unexpected mime type: %s", img.MIMEType)
	}
	if img.Format != "png" {
		t.Fatalf("unexpected format: %s", img.Format)
	}
	if img.Width != 0 || img.Height != 0 {
		t.Fatalf("expected zero dimensions without metadata, got %dx%d", img.Width, img.Height)
	}
}
