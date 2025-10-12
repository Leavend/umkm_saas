package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"server/internal/domain/jsoncfg"
	"server/internal/infra"
	"server/internal/infra/credentials"
	"server/internal/providers/genai"
	"server/internal/providers/image"
	"server/internal/providers/qwen"
	videoprovider "server/internal/providers/video"
	"server/internal/sqlinline"
	"server/internal/storage"
)

const (
	taskTypeImage = "IMAGE_GEN"
	taskTypeVideo = "VIDEO_GEN"

	statusSucceeded = "SUCCEEDED"
	statusFailed    = "FAILED"

	defaultImageProvider = "qwen-image-plus"
	defaultVideoProvider = "gemini-2.5-flash"

	jobPollInterval = 2 * time.Second

	sourceAssetDownloadTimeout = 30 * time.Second
)

const (
	maxSourceImageBytes int64 = 20 * 1024 * 1024
)

type job struct {
	ID       string
	UserID   string
	TaskType string
	Provider string
	Quantity int
	Aspect   string
	Prompt   json.RawMessage
}

type jobWorker struct {
	ctx            context.Context
	runner         *infra.SQLRunner
	logger         infra.Logger
	imageProviders map[string]image.Generator
	videoProviders map[string]videoprovider.Generator
	store          *storage.FileStore
	httpClient     *http.Client
}

var errNoJobAvailable = errors.New("no job available")

func main() {
	cfg, err := infra.LoadConfig()
	if err != nil {
		panic(err)
	}
	logger := infra.NewLogger(cfg.AppEnv)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := infra.NewDBPool(ctx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("worker: db connection failed")
	}
	defer pool.Close()

	runner := infra.NewSQLRunner(pool, logger)

	storagePath := cfg.StoragePath
	if storagePath == "" {
		storagePath = "./storage"
	}
	if !filepath.IsAbs(storagePath) {
		if abs, err := filepath.Abs(storagePath); err == nil {
			storagePath = abs
		}
	}
	fileStore, err := storage.NewFileStore(storagePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("worker: failed to configure storage")
	}

	credStore := credentials.NewStore(runner)

	qwenAPIKey := strings.TrimSpace(cfg.QwenAPIKey)
	if qwenAPIKey == "" {
		keyFromStore, err := credStore.QwenAPIKey(ctx)
		if err != nil {
			logger.Warn().Err(err).Msg("worker: failed to load qwen api key from store")
		} else {
			qwenAPIKey = keyFromStore
		}
	}

	geminiAPIKey := strings.TrimSpace(cfg.GeminiAPIKey)
	if geminiAPIKey == "" {
		keyFromStore, err := credStore.GeminiAPIKey(ctx)
		if err != nil {
			logger.Warn().Err(err).Msg("worker: failed to load gemini api key from store")
		} else {
			geminiAPIKey = keyFromStore
		}
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	geminiClient, err := genai.NewClient(genai.Options{
		APIKey:     geminiAPIKey,
		BaseURL:    cfg.GeminiBaseURL,
		Model:      cfg.GeminiModel,
		HTTPClient: httpClient,
		Logger:     &logger,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("worker: failed to configure gemini client")
	}

	if geminiAPIKey == "" {
		logger.Warn().Str("model", geminiClient.Model()).Msg("worker: gemini api key missing, using synthetic asset generation")
	}

	qwenClient, err := qwen.NewClient(qwen.Options{
		APIKey:         qwenAPIKey,
		BaseURL:        cfg.QwenBaseURL,
		Model:          cfg.QwenModel,
		DefaultSize:    cfg.QwenDefaultSize,
		PromptExtend:   true,
		Watermark:      false,
		HTTPClient:     httpClient,
		Logger:         &logger,
		RequestTimeout: 45 * time.Second,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("worker: failed to configure qwen client")
	}
	if !qwenClient.HasCredentials() {
		logger.Warn().Str("model", qwenClient.Model()).Msg("worker: qwen api key missing, falling back to synthetic assets")
	}

	worker := &jobWorker{
		ctx:            ctx,
		runner:         runner,
		logger:         logger,
		imageProviders: initImageProviders(qwenClient, geminiClient),
		videoProviders: initVideoProviders(geminiClient),
		store:          fileStore,
		httpClient:     httpClient,
	}

	if err := worker.Run(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatal().Err(err).Msg("worker: stopped with error")
	}
	logger.Info().Msg("worker: stopped")
}

func initImageProviders(qwenClient *qwen.Client, geminiClient *genai.Client) map[string]image.Generator {
	gemini := image.NewGeminiGenerator(geminiClient)
	qwen := image.NewQwenGenerator(qwenClient, gemini)
	providers := map[string]image.Generator{
		"qwen":             qwen,
		"qwen-image":       qwen,
		"qwen-image-plus":  qwen,
		"gemini":           gemini,
		"gemini-1.5-flash": gemini,
		"gemini-2.0-flash": gemini,
		"gemini-2.5-flash": gemini,
	}
	if qwenClient != nil {
		providers[strings.ToLower(qwenClient.Model())] = qwen
	}
	if geminiClient != nil {
		providers[strings.ToLower(geminiClient.Model())] = gemini
	}
	return providers
}

func initVideoProviders(client *genai.Client) map[string]videoprovider.Generator {
	gemini := videoprovider.NewGeminiGenerator(client)
	return map[string]videoprovider.Generator{
		"gemini":           gemini,
		"gemini-1.5-flash": gemini,
		"gemini-2.0-flash": gemini,
		"gemini-2.5-flash": gemini,
	}
}

func (w *jobWorker) Run() error {
	w.logger.Info().Msg("worker: started")
	for {
		select {
		case <-w.ctx.Done():
			return w.ctx.Err()
		default:
		}

		j, err := w.claimJob()
		if err != nil {
			if errors.Is(err, errNoJobAvailable) {
				time.Sleep(jobPollInterval)
				continue
			}
			w.logger.Error().Err(err).Msg("worker: failed to claim job")
			time.Sleep(jobPollInterval)
			continue
		}

		w.handleJob(j)
	}
}

func (w *jobWorker) handleJob(j job) {
	w.logger.Info().Str("job_id", j.ID).Str("task_type", j.TaskType).Msg("worker: picked job")
	status := statusFailed
	if err := w.dispatch(j); err != nil {
		w.logger.Error().Err(err).Str("job_id", j.ID).Msg("worker: job failed")
	} else {
		status = statusSucceeded
	}
	if err := w.updateStatus(j.ID, status); err != nil {
		w.logger.Error().Err(err).Str("job_id", j.ID).Msg("worker: update status failed")
	}
}

func (w *jobWorker) dispatch(j job) error {
	switch j.TaskType {
	case taskTypeImage:
		return w.processImageJob(j)
	case taskTypeVideo:
		return w.processVideoJob(j)
	default:
		return fmt.Errorf("unsupported job type %q", j.TaskType)
	}
}

func (w *jobWorker) claimJob() (job, error) {
	row := w.runner.QueryRow(w.ctx, sqlinline.QWorkerClaimJob)
	var j job
	if err := row.Scan(&j.ID, &j.UserID, &j.TaskType, &j.Provider, &j.Quantity, &j.Aspect, &j.Prompt); err != nil {
		if infra.IsNoRows(err) {
			return job{}, errNoJobAvailable
		}
		return job{}, err
	}
	// Ensure prompt bytes are not aliased.
	j.Prompt = append(json.RawMessage(nil), j.Prompt...)
	return j, nil
}

func (w *jobWorker) updateStatus(jobID, status string) error {
	_, err := w.runner.Exec(w.ctx, sqlinline.QUpdateJobStatus, jobID, status)
	return err
}

func (w *jobWorker) processImageJob(j job) error {
	var prompt jsoncfg.PromptJSON
	if err := json.Unmarshal(j.Prompt, &prompt); err != nil {
		return fmt.Errorf("decode image prompt: %w", err)
	}
	generator, provider := w.selectImageProvider(j.Provider)
	if generator == nil {
		return fmt.Errorf("image provider %q not configured", provider)
	}
	sourceImage, err := w.resolveSourceImage(j.UserID, prompt.SourceAsset)
	if err != nil {
		return fmt.Errorf("load source asset: %w", err)
	}
	workflow := image.Workflow{
		Mode:            image.NormalizeWorkflowMode(prompt.Workflow.Mode),
		BackgroundTheme: prompt.Workflow.BackgroundTheme,
		BackgroundStyle: prompt.Workflow.BackgroundStyle,
		EnhanceLevel:    prompt.Workflow.EnhanceLevel,
		RetouchStrength: prompt.Workflow.RetouchStrength,
		Notes:           prompt.Workflow.Notes,
	}
	assets, err := generator.Generate(w.ctx, image.GenerateRequest{
		Prompt:         image.BuildMarketingPrompt(prompt),
		Quantity:       j.Quantity,
		AspectRatio:    j.Aspect,
		Provider:       provider,
		RequestID:      j.ID,
		Locale:         prompt.Extras.Locale,
		WatermarkTag:   prompt.Watermark.Text,
		Quality:        prompt.Extras.Quality,
		NegativePrompt: image.DefaultNegativePrompt,
		Workflow:       workflow,
		SourceImage:    sourceImage,
	})
	if err != nil {
		return fmt.Errorf("image generation: %w", err)
	}
	for idx, asset := range assets {
		storageKey, size := w.persistAsset(j.ID, provider, asset.Format, asset.StorageKey, asset.URL, asset.Data, idx)
		if storageKey == "" {
			w.logger.Error().Str("job_id", j.ID).Msg("worker: image asset missing storage key")
			continue
		}
		metadata := map[string]any{"provider": provider}
		if asset.URL != "" && asset.URL != storageKey {
			metadata["source_url"] = asset.URL
		}
		if len(asset.Data) == 0 && size == 0 {
			size = 1024 * 1024
		}
		if _, execErr := w.runner.Exec(
			w.ctx,
			sqlinline.QInsertAsset,
			j.UserID,
			"GENERATED",
			j.ID,
			storageKey,
			asset.Format,
			size,
			asset.Width,
			asset.Height,
			j.Aspect,
			jsoncfg.MustMarshal(metadata),
		); execErr != nil {
			w.logger.Error().Err(execErr).Str("job_id", j.ID).Msg("worker: insert image asset failed")
		}
	}
	return nil
}

func (w *jobWorker) processVideoJob(j job) error {
	payload := map[string]any{}
	if len(j.Prompt) > 0 {
		if err := json.Unmarshal(j.Prompt, &payload); err != nil {
			return fmt.Errorf("decode video prompt: %w", err)
		}
	}
	generator, provider := w.selectVideoProvider(j.Provider)
	if generator == nil {
		return fmt.Errorf("video provider %q not configured", provider)
	}
	locale := ""
	if v, ok := payload["locale"].(string); ok {
		locale = v
	}
	asset, err := generator.Generate(w.ctx, videoprovider.GenerateRequest{
		Prompt:    extractPromptText(payload),
		Provider:  provider,
		RequestID: j.ID,
		Locale:    locale,
	})
	if err != nil {
		return fmt.Errorf("video generation: %w", err)
	}
	storageKey, size := w.persistAsset(j.ID, provider, asset.Format, asset.StorageKey, asset.URL, asset.Data, 0)
	if storageKey == "" {
		return fmt.Errorf("video asset missing storage key")
	}
	if size == 0 {
		size = int64(len(asset.Data))
	}
	if size == 0 {
		size = int64(5 * 1024 * 1024)
	}
	metadata := map[string]any{"provider": provider, "length": asset.Length}
	if asset.URL != "" && asset.URL != storageKey {
		metadata["source_url"] = asset.URL
	}
	if _, execErr := w.runner.Exec(
		w.ctx,
		sqlinline.QInsertAsset,
		j.UserID,
		"GENERATED",
		j.ID,
		storageKey,
		asset.Format,
		size,
		1920,
		1080,
		j.Aspect,
		jsoncfg.MustMarshal(metadata),
	); execErr != nil {
		w.logger.Error().Err(execErr).Str("job_id", j.ID).Msg("worker: insert video asset failed")
	}
	return nil
}

func (w *jobWorker) selectImageProvider(requested string) (image.Generator, string) {
	if generator, ok := w.imageProviders[requested]; ok {
		return generator, requested
	}
	generator, ok := w.imageProviders[defaultImageProvider]
	if !ok {
		return nil, requested
	}
	return generator, defaultImageProvider
}

func (w *jobWorker) selectVideoProvider(requested string) (videoprovider.Generator, string) {
	if generator, ok := w.videoProviders[requested]; ok {
		return generator, requested
	}
	generator, ok := w.videoProviders[defaultVideoProvider]
	if !ok {
		return nil, requested
	}
	return generator, defaultVideoProvider
}

func extractPromptText(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if text, ok := payload["prompt"].(string); ok {
		return text
	}
	if nested, ok := payload["prompt"].(map[string]any); ok {
		if text, ok := nested["text"].(string); ok {
			return text
		}
		if title, ok := nested["title"].(string); ok {
			return title
		}
	}
	return ""
}

func (w *jobWorker) persistAsset(jobID, provider, mime, storageKey, sourceURL string, data []byte, index int) (string, int64) {
	key := strings.TrimSpace(storageKey)
	if key == "" {
		key = strings.TrimSpace(sourceURL)
	}
	var size int64
	if len(data) > 0 {
		size = int64(len(data))
	}
	if w.store != nil && len(data) > 0 {
		targetKey := key
		if targetKey == "" || strings.HasPrefix(targetKey, "http://") || strings.HasPrefix(targetKey, "https://") {
			targetKey = defaultStorageKey(jobID, mime, index)
		}
		targetKey = ensureExtension(targetKey, mime)
		savedKey, err := w.store.Write(w.ctx, targetKey, data)
		if err != nil {
			w.logger.Warn().Err(err).
				Str("job_id", jobID).
				Str("provider", provider).
				Msg("worker: persist asset to storage failed")
		} else {
			key = savedKey
		}
	}
	return key, size
}

func defaultStorageKey(jobID, mime string, index int) string {
	category := "images"
	prefix := "image"
	if strings.HasPrefix(mime, "video/") {
		category = "videos"
		prefix = "video"
	}
	if index < 0 {
		index = 0
	}
	ext := extensionForMIME(mime)
	if ext == "" {
		ext = ".bin"
	}
	if category == "videos" {
		return fmt.Sprintf("generated/%s/%s/%s%s", category, jobID, prefix, ext)
	}
	return fmt.Sprintf("generated/%s/%s/%s-%02d%s", category, jobID, prefix, index+1, ext)
}

func ensureExtension(key, mime string) string {
	if key == "" {
		return key
	}
	expected := extensionForMIME(mime)
	if expected == "" {
		return key
	}
	ext := strings.ToLower(filepath.Ext(key))
	if ext == expected {
		return key
	}
	if ext == "" {
		return key + expected
	}
	return key
}

func extensionForMIME(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "video/mp4":
		return ".mp4"
	case "text/plain":
		return ".txt"
	default:
		return ""
	}
}

func (w *jobWorker) resolveSourceImage(userID string, cfg jsoncfg.SourceAssetConfig) (*image.SourceImage, error) {
	if cfg.IsZero() {
		return nil, nil
	}
	var (
		storageKey = strings.TrimSpace(cfg.StorageKey)
		mime       = strings.TrimSpace(cfg.Mime)
		filename   = strings.TrimSpace(cfg.Filename)
		sourceURL  = strings.TrimSpace(cfg.URL)
		width      int
		height     int
	)
	if cfg.AssetID != "" {
		row := w.runner.QueryRow(w.ctx, sqlinline.QSelectAssetByID, cfg.AssetID)
		var (
			assetID    string
			ownerID    string
			storedKey  string
			storedMIME string
			bytes      int64
			storedW    int
			storedH    int
			aspect     string
			props      []byte
		)
		if err := row.Scan(&assetID, &ownerID, &storedKey, &storedMIME, &bytes, &storedW, &storedH, &aspect, &props); err != nil {
			return nil, err
		}
		if ownerID != userID {
			return nil, fmt.Errorf("source asset %s does not belong to user", cfg.AssetID)
		}
		if storageKey == "" {
			storageKey = storedKey
		}
		if mime == "" {
			mime = storedMIME
		}
		if width == 0 {
			width = storedW
		}
		if height == 0 {
			height = storedH
		}
		if sourceURL == "" {
			var meta map[string]any
			if err := json.Unmarshal(props, &meta); err == nil {
				if val, ok := meta["source_url"].(string); ok {
					sourceURL = val
				} else if val, ok := meta["url"].(string); ok {
					sourceURL = val
				}
			}
		}
		if filename == "" {
			if val, ok := jsonPropertyString(props, "filename"); ok {
				filename = val
			}
		}
	}
	var data []byte
	if storageKey != "" && !isRemotePath(storageKey) && w.store != nil {
		saved, err := w.store.Read(w.ctx, storageKey)
		if err == nil {
			data = saved
		} else {
			w.logger.Warn().Err(err).Str("storage_key", storageKey).Msg("worker: failed to read source asset from storage")
		}
	}
	if filename == "" && storageKey != "" {
		filename = filepath.Base(storageKey)
	}
	if sourceURL == "" {
		sourceURL = strings.TrimSpace(cfg.URL)
	}
	if len(data) == 0 && sourceURL != "" {
		if fetched, fetchedMIME := w.fetchSourceAsset(sourceURL); len(fetched) > 0 {
			data = fetched
			if mime == "" {
				mime = fetchedMIME
			}
		}
	}
	if filename == "" && sourceURL != "" {
		if parsed, err := neturl.Parse(sourceURL); err == nil {
			if base := filepath.Base(parsed.Path); base != "" && base != "." {
				filename = base
			}
		} else {
			if base := filepath.Base(sourceURL); base != "" && base != "." {
				filename = base
			}
		}
	}
	return &image.SourceImage{
		AssetID:    strings.TrimSpace(cfg.AssetID),
		StorageKey: storageKey,
		URL:        sourceURL,
		MIME:       mime,
		Data:       data,
		Width:      width,
		Height:     height,
		Filename:   filename,
	}, nil
}

func (w *jobWorker) fetchSourceAsset(sourceURL string) ([]byte, string) {
	if w.httpClient == nil {
		return nil, ""
	}
	trimmed := strings.TrimSpace(sourceURL)
	if trimmed == "" {
		return nil, ""
	}
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return nil, ""
	}
	downloadCtx, cancel := context.WithTimeout(w.ctx, sourceAssetDownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, trimmed, nil)
	if err != nil {
		w.logger.Warn().Err(err).Str("url", trimmed).Msg("worker: build source asset request failed")
		return nil, ""
	}
	resp, err := w.httpClient.Do(req)
	if err != nil {
		w.logger.Warn().Err(err).Str("url", trimmed).Msg("worker: download source asset failed")
		return nil, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		w.logger.Warn().Int("status", resp.StatusCode).Str("url", trimmed).Msg("worker: source asset responded with non-success status")
		return nil, ""
	}
	limited := io.LimitReader(resp.Body, maxSourceImageBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		w.logger.Warn().Err(err).Str("url", trimmed).Msg("worker: read source asset failed")
		return nil, ""
	}
	if int64(len(data)) > maxSourceImageBytes {
		w.logger.Warn().Int64("bytes", int64(len(data))).Str("url", trimmed).Msg("worker: source asset exceeds max size, falling back to url")
		return nil, ""
	}
	mime := strings.TrimSpace(resp.Header.Get("Content-Type"))
	return data, mime
}

func jsonPropertyString(raw []byte, key string) (string, bool) {
	if len(raw) == 0 {
		return "", false
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", false
	}
	if val, ok := payload[key].(string); ok {
		return strings.TrimSpace(val), true
	}
	return "", false
}

func isRemotePath(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:")
}
