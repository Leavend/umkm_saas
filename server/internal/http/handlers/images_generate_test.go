package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"server/internal/db"
	"server/internal/imagegen"
	"server/internal/infra"
	"server/internal/middleware"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog"
)

type stubRow struct {
	scan func(dest ...any) error
}

func (r stubRow) Scan(dest ...any) error {
	if r.scan == nil {
		return errors.New("no row")
	}
	return r.scan(dest...)
}

type stubDB struct {
	mu   sync.Mutex
	jobs map[uuid.UUID]*db.ImageJob
}

func newStubDB() *stubDB {
	return &stubDB{jobs: make(map[uuid.UUID]*db.ImageJob)}
}

func (s *stubDB) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case strings.Contains(query, "SET status = 'RUNNING'"):
		id := args[0].(uuid.UUID)
		job := s.jobs[id]
		if job == nil {
			return pgconn.CommandTag{}, errors.New("job not found")
		}
		job.Status = "RUNNING"
		job.UpdatedAt = time.Now()
	case strings.Contains(query, "SET status = 'SUCCEEDED'"):
		id := args[0].(uuid.UUID)
		job := s.jobs[id]
		if job == nil {
			return pgconn.CommandTag{}, errors.New("job not found")
		}
		job.Status = "SUCCEEDED"
		if output, ok := args[1].([]byte); ok {
			job.Output = append([]byte(nil), output...)
		}
		job.UpdatedAt = time.Now()
	case strings.Contains(query, "SET status = 'FAILED'"):
		id := args[0].(uuid.UUID)
		job := s.jobs[id]
		if job == nil {
			return pgconn.CommandTag{}, errors.New("job not found")
		}
		job.Status = "FAILED"
		if msg, ok := args[1].(string); ok {
			job.Error = sql.NullString{String: msg, Valid: true}
		}
		job.UpdatedAt = time.Now()
	default:
		return pgconn.CommandTag{}, fmt.Errorf("unsupported exec: %s", query)
	}
	return pgconn.CommandTag{}, nil
}

func (s *stubDB) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return nil, fmt.Errorf("unsupported query: %s", query)
}

func (s *stubDB) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, "INSERT INTO image_jobs") {
		id := uuid.New()
		job := &db.ImageJob{
			ID:          id,
			Provider:    args[1].(string),
			Model:       args[2].(string),
			Status:      "QUEUED",
			Quantity:    args[3].(int32),
			Prompt:      append([]byte(nil), args[5].([]byte)...),
			SourceAsset: append([]byte(nil), args[6].([]byte)...),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if ptr, ok := args[0].(*string); ok && ptr != nil {
			job.UserID = sql.NullString{String: *ptr, Valid: true}
		}
		if ptr, ok := args[4].(*string); ok && ptr != nil {
			job.AspectRatio = sql.NullString{String: *ptr, Valid: true}
		}
		s.mu.Lock()
		s.jobs[id] = job
		s.mu.Unlock()
		return stubRow{scan: func(dest ...any) error {
			if len(dest) == 0 {
				return nil
			}
			if ptr, ok := dest[0].(*uuid.UUID); ok && ptr != nil {
				*ptr = id
				return nil
			}
			return fmt.Errorf("unsupported scan target")
		}}
	}
	return stubRow{scan: func(dest ...any) error {
		return fmt.Errorf("unsupported query: %s", query)
	}}
}

func (s *stubDB) lastJob() *db.ImageJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.jobs {
		copy := *job
		return &copy
	}
	return nil
}

type stubEditor struct {
	mu    sync.Mutex
	urls  []string
	err   error
	calls int
}

func (s *stubEditor) EditOnce(ctx context.Context, imageURL, instruction string, watermark bool, negative string, seed *int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	if len(s.urls) >= s.calls {
		return s.urls[s.calls-1], nil
	}
	return fmt.Sprintf("https://example.com/generated-%d.png", s.calls), nil
}

func TestImagesGenerateHandler(t *testing.T) {
	testCases := []struct {
		name       string
		body       map[string]any
		editor     func() *stubEditor
		wantStatus int
		wantImages int
		wantJob    string
	}{{
		name: "success",
		editor: func() *stubEditor {
			return &stubEditor{urls: []string{"https://example.com/a.png", "https://example.com/b.png"}}
		},
		wantStatus: http.StatusCreated,
		wantImages: 2,
		wantJob:    "SUCCEEDED",
		body: map[string]any{
			"provider":     "qwen-image-plus",
			"quantity":     2,
			"aspect_ratio": "1:1",
			"prompt": map[string]any{
				"title":        "Sample",
				"product_type": "food",
				"style":        "modern",
				"background":   "studio",
				"instructions": "bright",
				"watermark":    map[string]any{"enabled": false},
				"source_asset": map[string]any{"asset_id": "upl", "url": "https://example.com/source.png"},
			},
		},
	}, {
		name:       "missing source",
		editor:     func() *stubEditor { return &stubEditor{urls: []string{"https://example.com/one.png"}} },
		wantStatus: http.StatusUnprocessableEntity,
		wantImages: 0,
		wantJob:    "",
		body: map[string]any{
			"provider": "qwen-image-plus",
			"quantity": 1,
			"prompt": map[string]any{
				"title":        "Sample",
				"watermark":    map[string]any{"enabled": false},
				"source_asset": map[string]any{"asset_id": "upl", "url": ""},
			},
		},
	}, {
		name:       "editor failure",
		editor:     func() *stubEditor { return &stubEditor{err: errors.New("generation failed")} },
		wantStatus: http.StatusBadGateway,
		wantImages: 0,
		wantJob:    "FAILED",
		body: map[string]any{
			"provider": "qwen-image-edit",
			"quantity": 1,
			"prompt": map[string]any{
				"title":        "Sample",
				"watermark":    map[string]any{"enabled": false},
				"source_asset": map[string]any{"asset_id": "upl", "url": "https://example.com/source.png"},
			},
		},
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dbStub := newStubDB()
			editor := tc.editor()

			app := &App{
				Config:       &infra.Config{},
				Logger:       zerolog.Nop(),
				DB:           dbStub,
				ImageEditor:  editor,
				imageLimiter: make(chan struct{}, 2),
			}

			bodyBytes, err := json.Marshal(tc.body)
			if err != nil {
				t.Fatalf("failed to marshal body: %v", err)
			}
			req := httptest.NewRequest("POST", "/v1/images/generate", bytes.NewReader(bodyBytes))
			req = req.WithContext(middleware.ContextWithUserID(req.Context(), "user-123"))
			rr := httptest.NewRecorder()

			app.ImagesGenerate(rr, req)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rr.Code, tc.wantStatus, rr.Body.String())
			}

			if tc.wantImages > 0 {
				var resp imagegen.GenerateResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if len(resp.Images) != tc.wantImages {
					t.Fatalf("images len = %d, want %d", len(resp.Images), tc.wantImages)
				}
			}

			job := dbStub.lastJob()
			if tc.wantJob == "" {
				if job != nil {
					t.Fatalf("expected no job recorded")
				}
			} else {
				if job == nil {
					t.Fatalf("expected job to be created")
				}
				if job.Status != tc.wantJob {
					t.Fatalf("job status = %s, want %s", job.Status, tc.wantJob)
				}
				if tc.wantJob == "SUCCEEDED" && len(job.Output) == 0 {
					t.Fatalf("expected job output recorded")
				}
				if tc.wantJob == "FAILED" && (!job.Error.Valid || job.Error.String == "") {
					t.Fatalf("expected failure recorded")
				}
			}
		})
	}
}
