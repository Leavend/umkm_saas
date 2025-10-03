package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"server/internal/domain/jsoncfg"
	handlers "server/internal/http/handlers"
	"server/internal/http/httpapi"
	"server/internal/infra"
	"server/internal/middleware"
	"server/internal/providers/image"
	videoprovider "server/internal/providers/video"
	"server/internal/sqlinline"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestImagesGenerateIntegration(t *testing.T) {
        ctx := context.Background()
        runner := newFakeSQLRunner()
        userID := uuid.NewString()
        runner.addUser(userID, 2)

	cfg := &infra.Config{
		AppEnv:          "test",
		JWTSecret:       "test-secret",
		RateLimitPerMin: 100,
	}
	logger := infra.NewLogger("test")
	app := &handlers.App{
		Config:         cfg,
		Logger:         logger,
		SQL:            runner,
		ImageProviders: map[string]image.Generator{"gemini": image.NewNanoBanana()},
		VideoProviders: map[string]videoprovider.Generator{},
		JWTSecret:      cfg.JWTSecret,
	}

	router := httpapi.NewRouter(app)

	reqPayload := imageGenerateReq{
		Provider:    "",
		Quantity:    0,
		AspectRatio: "",
		Prompt: jsoncfg.PromptJSON{
			Title:        "Nasi Goreng",
			ProductType:  "food",
			Style:        "elegan",
			Background:   "studio_white",
			Instructions: "tampilkan plating mewah",
			Watermark:    jsoncfg.WatermarkConfig{Enabled: false},
			References:   []string{},
			Extras:       jsoncfg.ExtrasConfig{},
		},
	}
	body, err := json.Marshal(reqPayload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	token, err := middleware.SignJWT(cfg.JWTSecret, middleware.TokenClaims{
		Sub:      userID,
		Plan:     "free",
		Locale:   "id",
		Exp:      time.Now().Add(time.Hour).Unix(),
		Issuer:   "integration-test",
		Audience: "client-test",
	})
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("/v1/images/generate status = %d, want %d", res.Code, http.StatusAccepted)
	}
	var jobRes jobResponseDTO
	if err := json.Unmarshal(res.Body.Bytes(), &jobRes); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if jobRes.JobID == "" {
		t.Fatalf("expected job id in response")
	}
	if jobRes.RemainingQuota != 1 {
		t.Fatalf("remaining quota = %d, want %d", jobRes.RemainingQuota, 1)
	}

	user := runner.getUser(userID)
	if user == nil {
		t.Fatalf("user not found in runner state")
	}
	if user.QuotaUsed != 1 {
		t.Fatalf("quota used = %d, want %d", user.QuotaUsed, 1)
	}

	// Simulate worker consumption for the queued job.
	row := runner.QueryRow(ctx, sqlinline.QWorkerClaimJob)
	var (
		jobID      string
		claimed    string
		taskType   string
		provider   string
		quantity   int
		aspect     string
		promptJSON []byte
	)
	if err := row.Scan(&jobID, &claimed, &taskType, &provider, &quantity, &aspect, &promptJSON); err != nil {
		t.Fatalf("claim job scan: %v", err)
	}
	if jobID != jobRes.JobID {
		t.Fatalf("claimed job = %s, want %s", jobID, jobRes.JobID)
	}
	var prompt jsoncfg.PromptJSON
	if err := json.Unmarshal(promptJSON, &prompt); err != nil {
		t.Fatalf("unmarshal prompt: %v", err)
	}
	generator := app.ImageProviders[provider]
	assets, err := generator.Generate(ctx, image.GenerateRequest{
		Prompt:       prompt.Title,
		Quantity:     quantity,
		AspectRatio:  aspect,
		Provider:     provider,
		RequestID:    jobID,
		Locale:       prompt.Extras.Locale,
		WatermarkTag: prompt.Watermark.Text,
	})
	if err != nil {
		t.Fatalf("generate assets: %v", err)
	}
	for _, asset := range assets {
		if _, execErr := runner.Exec(ctx, sqlinline.QInsertAsset, claimed, "GENERATED", jobID, asset.URL, asset.Format, int64(1024*1024), asset.Width, asset.Height, aspect, jsoncfg.MustMarshal(map[string]any{"provider": provider})); execErr != nil {
			t.Fatalf("insert asset: %v", execErr)
		}
	}
	if _, err := runner.Exec(ctx, sqlinline.QUpdateJobStatus, jobID, "SUCCEEDED"); err != nil {
		t.Fatalf("update job status: %v", err)
	}

	job := runner.getJob(jobID)
	if job == nil {
		t.Fatalf("job not found")
	}
	if job.Status != "SUCCEEDED" {
		t.Fatalf("job status = %s, want %s", job.Status, "SUCCEEDED")
	}
	if len(job.Assets) != quantity {
		t.Fatalf("asset count = %d, want %d", len(job.Assets), quantity)
	}
}

func TestImageJobAccessControl(t *testing.T) {
        runner := newFakeSQLRunner()
        ownerID := uuid.NewString()
        otherID := uuid.NewString()
        runner.addUser(ownerID, 2)
        runner.addUser(otherID, 2)

        cfg := &infra.Config{
                AppEnv:          "test",
                JWTSecret:       "test-secret",
                RateLimitPerMin: 100,
        }
        logger := infra.NewLogger("test")
        app := &handlers.App{
                Config:         cfg,
                Logger:         logger,
                SQL:            runner,
                ImageProviders: map[string]image.Generator{"gemini": image.NewNanoBanana()},
                VideoProviders: map[string]videoprovider.Generator{},
                JWTSecret:      cfg.JWTSecret,
        }
        router := httpapi.NewRouter(app)

        reqPayload := imageGenerateReq{
                Prompt: jsoncfg.PromptJSON{Title: "Ayam Bakar"},
        }
        body, err := json.Marshal(reqPayload)
        if err != nil {
                t.Fatalf("marshal payload: %v", err)
        }
        generateReq := httptest.NewRequest(http.MethodPost, "/v1/images/generate", bytes.NewReader(body))
        generateReq.Header.Set("Content-Type", "application/json")
        ownerToken := newToken(t, cfg.JWTSecret, ownerID)
        generateReq.Header.Set("Authorization", "Bearer "+ownerToken)

        generateRes := httptest.NewRecorder()
        router.ServeHTTP(generateRes, generateReq)

        if generateRes.Code != http.StatusAccepted {
                t.Fatalf("generate status = %d, want %d", generateRes.Code, http.StatusAccepted)
        }

        var jobRes jobResponseDTO
        if err := json.Unmarshal(generateRes.Body.Bytes(), &jobRes); err != nil {
                t.Fatalf("decode response: %v", err)
        }

        statusReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/images/%s/status", jobRes.JobID), nil)
        statusReq.Header.Set("Authorization", "Bearer "+ownerToken)
        statusRes := httptest.NewRecorder()
        router.ServeHTTP(statusRes, statusReq)
        if statusRes.Code != http.StatusOK {
                t.Fatalf("owner status = %d, want %d", statusRes.Code, http.StatusOK)
        }

        otherToken := newToken(t, cfg.JWTSecret, otherID)

        unauthorizedStatus := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/images/%s/status", jobRes.JobID), nil)
        unauthorizedStatus.Header.Set("Authorization", "Bearer "+otherToken)
        unauthorizedStatusRes := httptest.NewRecorder()
        router.ServeHTTP(unauthorizedStatusRes, unauthorizedStatus)
        if unauthorizedStatusRes.Code != http.StatusNotFound {
                t.Fatalf("other user status = %d, want %d", unauthorizedStatusRes.Code, http.StatusNotFound)
        }

        unauthorizedAssets := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/images/%s/assets", jobRes.JobID), nil)
        unauthorizedAssets.Header.Set("Authorization", "Bearer "+otherToken)
        unauthorizedAssetsRes := httptest.NewRecorder()
        router.ServeHTTP(unauthorizedAssetsRes, unauthorizedAssets)
        if unauthorizedAssetsRes.Code != http.StatusNotFound {
                t.Fatalf("other user assets = %d, want %d", unauthorizedAssetsRes.Code, http.StatusNotFound)
        }

        unauthorizedZip := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/images/%s/zip", jobRes.JobID), nil)
        unauthorizedZip.Header.Set("Authorization", "Bearer "+otherToken)
        unauthorizedZipRes := httptest.NewRecorder()
        router.ServeHTTP(unauthorizedZipRes, unauthorizedZip)
        if unauthorizedZipRes.Code != http.StatusNotFound {
                t.Fatalf("other user zip = %d, want %d", unauthorizedZipRes.Code, http.StatusNotFound)
        }
}

func newToken(t *testing.T, secret, userID string) string {
        t.Helper()
        token, err := middleware.SignJWT(secret, middleware.TokenClaims{
                Sub:      userID,
                Plan:     "free",
                Locale:   "id",
                Exp:      time.Now().Add(time.Hour).Unix(),
                Issuer:   "integration-test",
                Audience: "client-test",
        })
        if err != nil {
                t.Fatalf("sign jwt: %v", err)
        }
        return token
}

type imageGenerateReq struct {
	Provider    string             `json:"provider"`
	Quantity    int                `json:"quantity"`
	AspectRatio string             `json:"aspect_ratio"`
	Prompt      jsoncfg.PromptJSON `json:"prompt"`
}

type jobResponseDTO struct {
	JobID          string `json:"job_id"`
	Status         string `json:"status"`
	RemainingQuota int    `json:"remaining_quota"`
}

type fakeSQLRunner struct {
	mu       sync.Mutex
	users    map[string]*testUser
	jobs     map[string]*testJob
	jobOrder []string
	assetSeq int
	jobSeq   int
}

type testUser struct {
	ID         string
	QuotaDaily int
	QuotaUsed  int
}

type testJob struct {
        ID        string
        UserID    string
        TaskType  string
        Provider  string
        Quantity  int
        Aspect    string
        Prompt    []byte
        Status    string
        Assets    []testAsset
        CreatedAt time.Time
        UpdatedAt time.Time
        Props     []byte
}

type testAsset struct {
	ID        string
	UserID    string
	RequestID string
	Storage   string
	MIME      string
	Bytes     int64
	Width     int
	Height    int
	Aspect    string
	Props     []byte
	CreatedAt time.Time
}

func newFakeSQLRunner() *fakeSQLRunner {
	return &fakeSQLRunner{
		users: make(map[string]*testUser),
		jobs:  make(map[string]*testJob),
	}
}

func (f *fakeSQLRunner) addUser(id string, quotaDaily int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[id] = &testUser{ID: id, QuotaDaily: quotaDaily}
}

func (f *fakeSQLRunner) getUser(id string) *testUser {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.users[id]
}

func (f *fakeSQLRunner) getJob(id string) *testJob {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.jobs[id]
}

func (f *fakeSQLRunner) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch query {
	case sqlinline.QInsertAsset:
		if len(args) != 10 {
			return pgconn.CommandTag{}, fmt.Errorf("unexpected args for insert asset: %d", len(args))
		}
		userID, _ := args[0].(string)
		requestID, _ := args[2].(string)
		storage, _ := args[3].(string)
		mime, _ := args[4].(string)
		bytes, _ := args[5].(int64)
		width, _ := args[6].(int)
		height, _ := args[7].(int)
		aspect, _ := args[8].(string)
		props, _ := args[9].([]byte)
		f.assetSeq++
		asset := testAsset{
			ID:        fmt.Sprintf("asset-%d", f.assetSeq),
			UserID:    userID,
			RequestID: requestID,
			Storage:   storage,
			MIME:      mime,
			Bytes:     bytes,
			Width:     width,
			Height:    height,
			Aspect:    aspect,
			Props:     append([]byte(nil), props...),
			CreatedAt: time.Now(),
		}
		if job, ok := f.jobs[requestID]; ok {
			job.Assets = append(job.Assets, asset)
		}
		return pgconn.CommandTag{}, nil
	case sqlinline.QUpdateJobStatus:
		if len(args) != 2 {
			return pgconn.CommandTag{}, fmt.Errorf("unexpected args for update status: %d", len(args))
		}
		jobID, _ := args[0].(string)
		status, _ := args[1].(string)
		job, ok := f.jobs[jobID]
		if !ok {
			return pgconn.CommandTag{}, fmt.Errorf("job not found: %s", jobID)
		}
		job.Status = status
		job.UpdatedAt = time.Now()
		return pgconn.CommandTag{}, nil
	default:
		return pgconn.CommandTag{}, fmt.Errorf("unexpected exec query: %s", query)
	}
}

func (f *fakeSQLRunner) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch query {
	case sqlinline.QEnqueueImageJob:
		if len(args) != 5 {
			return pgx.SimpleRow{ScanFunc: func(dest ...any) error { return fmt.Errorf("unexpected args") }}
		}
		userID, _ := args[0].(string)
		prompt, _ := args[1].([]byte)
		quantity, _ := args[2].(int)
		aspect, _ := args[3].(string)
		provider, _ := args[4].(string)
		user, ok := f.users[userID]
		if !ok {
			return pgx.SimpleRow{ScanFunc: func(dest ...any) error { return fmt.Errorf("user not found") }}
		}
		if quantity <= 0 {
			quantity = jsoncfg.DefaultPromptQuantity
		}
		if quantity > jsoncfg.MaxPromptQuantity {
			quantity = jsoncfg.MaxPromptQuantity
		}
		if user.QuotaUsed+quantity > user.QuotaDaily {
			return pgx.SimpleRow{ScanFunc: func(dest ...any) error { return fmt.Errorf("quota exceeded") }}
		}
		user.QuotaUsed += quantity
		f.jobSeq++
		jobID := fmt.Sprintf("job-%d", f.jobSeq)
                job := &testJob{
                        ID:        jobID,
                        UserID:    userID,
                        TaskType:  "IMAGE_GEN",
                        Provider:  provider,
                        Quantity:  quantity,
                        Aspect:    aspect,
                        Prompt:    append([]byte(nil), prompt...),
                        Status:    "QUEUED",
                        CreatedAt: time.Now(),
                        UpdatedAt: time.Now(),
                        Props:     []byte("{}"),
                }
                f.jobs[jobID] = job
                f.jobOrder = append(f.jobOrder, jobID)
                remaining := user.QuotaDaily - user.QuotaUsed
		return pgx.SimpleRow{ScanFunc: func(dest ...any) error {
			if len(dest) != 2 {
				return fmt.Errorf("unexpected scan args: %d", len(dest))
			}
			if idPtr, ok := dest[0].(*string); ok {
				*idPtr = jobID
			} else {
				return fmt.Errorf("job id dest must be *string")
			}
			switch v := dest[1].(type) {
			case *int:
				*v = remaining
			case *int32:
				*v = int32(remaining)
			case *int64:
				*v = int64(remaining)
			default:
				return fmt.Errorf("remaining dest type unsupported")
			}
			return nil
		}}
        case sqlinline.QSelectJobStatus:
                if len(args) != 2 {
                        return pgx.SimpleRow{ScanFunc: func(dest ...any) error { return fmt.Errorf("unexpected args") }}
                }
                jobID, _ := args[0].(string)
                userID, _ := args[1].(string)
                job, ok := f.jobs[jobID]
                if !ok || job.UserID != userID {
                        return pgx.SimpleRow{ScanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
                }
                jobCopy := *job
                propsCopy := append([]byte(nil), jobCopy.Props...)
                return pgx.SimpleRow{ScanFunc: func(dest ...any) error {
                        if len(dest) != 10 {
                                return fmt.Errorf("unexpected scan args: %d", len(dest))
                        }
                        if v, ok := dest[0].(*string); ok {
                                *v = jobCopy.ID
                        } else {
                                return fmt.Errorf("dest[0] not *string")
                        }
                        if v, ok := dest[1].(*string); ok {
                                *v = jobCopy.UserID
                        } else {
                                return fmt.Errorf("dest[1] not *string")
                        }
                        if v, ok := dest[2].(*string); ok {
                                *v = jobCopy.TaskType
                        } else {
                                return fmt.Errorf("dest[2] not *string")
                        }
                        if v, ok := dest[3].(*string); ok {
                                *v = jobCopy.Status
                        } else {
                                return fmt.Errorf("dest[3] not *string")
                        }
                        if v, ok := dest[4].(*string); ok {
                                *v = jobCopy.Provider
                        } else {
                                return fmt.Errorf("dest[4] not *string")
                        }
                        switch v := dest[5].(type) {
                        case *int:
                                *v = jobCopy.Quantity
                        case *int32:
                                *v = int32(jobCopy.Quantity)
                        case *int64:
                                *v = int64(jobCopy.Quantity)
                        default:
                                return fmt.Errorf("dest[5] unsupported type")
                        }
                        if v, ok := dest[6].(*string); ok {
                                *v = jobCopy.Aspect
                        } else {
                                return fmt.Errorf("dest[6] not *string")
                        }
                        if v, ok := dest[7].(*time.Time); ok {
                                *v = jobCopy.CreatedAt
                        } else {
                                return fmt.Errorf("dest[7] not *time.Time")
                        }
                        if v, ok := dest[8].(*time.Time); ok {
                                *v = jobCopy.UpdatedAt
                        } else {
                                return fmt.Errorf("dest[8] not *time.Time")
                        }
                        if v, ok := dest[9].(*[]byte); ok {
                                *v = append([]byte(nil), propsCopy...)
                        } else {
                                return fmt.Errorf("dest[9] not *[]byte")
                        }
                        return nil
                }}
        case sqlinline.QWorkerClaimJob:
                for _, id := range f.jobOrder {
                        job := f.jobs[id]
                        if job.Status != "QUEUED" {
                                continue
			}
			job.Status = "RUNNING"
			job.UpdatedAt = time.Now()
			promptCopy := append([]byte(nil), job.Prompt...)
			return pgx.SimpleRow{ScanFunc: func(dest ...any) error {
				if len(dest) != 7 {
					return fmt.Errorf("unexpected scan args: %d", len(dest))
				}
				if v, ok := dest[0].(*string); ok {
					*v = job.ID
				} else {
					return fmt.Errorf("dest[0] not *string")
				}
				if v, ok := dest[1].(*string); ok {
					*v = job.UserID
				} else {
					return fmt.Errorf("dest[1] not *string")
				}
				if v, ok := dest[2].(*string); ok {
					*v = job.TaskType
				} else {
					return fmt.Errorf("dest[2] not *string")
				}
				if v, ok := dest[3].(*string); ok {
					*v = job.Provider
				} else {
					return fmt.Errorf("dest[3] not *string")
				}
				if v, ok := dest[4].(*int); ok {
					*v = job.Quantity
				} else {
					return fmt.Errorf("dest[4] not *int")
				}
				if v, ok := dest[5].(*string); ok {
					*v = job.Aspect
				} else {
					return fmt.Errorf("dest[5] not *string")
				}
				if v, ok := dest[6].(*[]byte); ok {
					*v = promptCopy
				} else {
					return fmt.Errorf("dest[6] not *[]byte")
				}
				return nil
			}}
		}
		return pgx.SimpleRow{ScanFunc: func(dest ...any) error { return pgx.ErrNoRows }}
	default:
		return pgx.SimpleRow{ScanFunc: func(dest ...any) error { return fmt.Errorf("unexpected query: %s", query) }}
	}
}

func (f *fakeSQLRunner) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
        f.mu.Lock()
        defer f.mu.Unlock()
        switch query {
        case sqlinline.QSelectJobAssets:
                if len(args) != 2 {
                        return nil, fmt.Errorf("unexpected args for select assets: %d", len(args))
                }
                jobID, _ := args[0].(string)
                userID, _ := args[1].(string)
                job, ok := f.jobs[jobID]
                if !ok || job.UserID != userID {
                        return &fakeRows{}, nil
                }
                assets := make([]testAsset, len(job.Assets))
                copy(assets, job.Assets)
                return &fakeRows{items: assets}, nil
        default:
                return nil, fmt.Errorf("unexpected query: %s", query)
        }
}

type fakeRows struct {
        items []testAsset
        idx   int
}

func (r *fakeRows) Next() bool {
        if r.idx >= len(r.items) {
                return false
        }
        r.idx++
        return true
}

func (r *fakeRows) Scan(dest ...any) error {
        if r.idx == 0 || r.idx > len(r.items) {
                return pgx.ErrNoRows
        }
        asset := r.items[r.idx-1]
        if len(dest) != 9 {
                return fmt.Errorf("unexpected scan args: %d", len(dest))
        }
        if v, ok := dest[0].(*string); ok {
                *v = asset.ID
        } else {
                return fmt.Errorf("dest[0] not *string")
        }
        if v, ok := dest[1].(*string); ok {
                *v = asset.Storage
        } else {
                return fmt.Errorf("dest[1] not *string")
        }
        if v, ok := dest[2].(*string); ok {
                *v = asset.MIME
        } else {
                return fmt.Errorf("dest[2] not *string")
        }
        switch v := dest[3].(type) {
        case *int64:
                *v = asset.Bytes
        case *int:
                *v = int(asset.Bytes)
        default:
                return fmt.Errorf("dest[3] unsupported type")
        }
        switch v := dest[4].(type) {
        case *int:
                *v = asset.Width
        case *int32:
                *v = int32(asset.Width)
        default:
                return fmt.Errorf("dest[4] unsupported type")
        }
        switch v := dest[5].(type) {
        case *int:
                *v = asset.Height
        case *int32:
                *v = int32(asset.Height)
        default:
                return fmt.Errorf("dest[5] unsupported type")
        }
        if v, ok := dest[6].(*string); ok {
                *v = asset.Aspect
        } else {
                return fmt.Errorf("dest[6] not *string")
        }
        if v, ok := dest[7].(*[]byte); ok {
                *v = append([]byte(nil), asset.Props...)
        } else {
                return fmt.Errorf("dest[7] not *[]byte")
        }
        if v, ok := dest[8].(*time.Time); ok {
                *v = asset.CreatedAt
        } else {
                return fmt.Errorf("dest[8] not *time.Time")
        }
        return nil
}

func (r *fakeRows) Err() error {
        return nil
}

func (r *fakeRows) Close() {}
