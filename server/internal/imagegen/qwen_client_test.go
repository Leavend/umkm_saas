package imagegen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	got, err := client.EditOnce(context.Background(), "https://example.com/in.png", "do something", true, "", nil)
	if err != nil {
		t.Fatalf("EditOnce error: %v", err)
	}
	if got != "https://example.com/out.png" {
		t.Fatalf("unexpected url: %s", got)
	}
}

func TestQwenClientMissingKey(t *testing.T) {
	client := NewQwenClient(QwenOptions{})
	if _, err := client.EditOnce(context.Background(), "https://example.com/in.png", "instr", false, "", nil); err == nil {
		t.Fatalf("expected error when api key missing")
	}
}
