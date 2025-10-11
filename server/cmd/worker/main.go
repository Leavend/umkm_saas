package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	videoprovider "server/internal/providers/video"
	"server/internal/sqlinline"
	"server/internal/storage"
)

const (
	taskTypeImage = "IMAGE_GEN"
	taskTypeVideo = "VIDEO_GEN"

	statusSucceeded = "SUCCEEDED"
	statusFailed    = "FAILED"

	defaultImageProvider = "gemini-2.5-flash"
	defaultVideoProvider = "gemini-2.5-flash"

	jobPollInterval = 2 * time.Second
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

	geminiAPIKey := strings.TrimSpace(cfg.GeminiAPIKey)
	if geminiAPIKey == "" {
		credStore := credentials.NewStore(runner)
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

	worker := &jobWorker{
		ctx:            ctx,
		runner:         runner,
		logger:         logger,
		imageProviders: initImageProviders(geminiClient),
		videoProviders: initVideoProviders(geminiClient),
		store:          fileStore,
	}

	if err := worker.Run(); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatal().Err(err).Msg("worker: stopped with error")
	}
	logger.Info().Msg("worker: stopped")
}

func initImageProviders(client *genai.Client) map[string]image.Generator {
	gemini := image.NewGeminiGenerator(client)
	return map[string]image.Generator{
		"gemini":           gemini,
		"gemini-1.5-flash": gemini,
		"gemini-2.0-flash": gemini,
		"gemini-2.5-flash": gemini,
	}
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
	assets, err := generator.Generate(w.ctx, image.GenerateRequest{
		Prompt:       prompt.Title,
		Quantity:     j.Quantity,
		AspectRatio:  j.Aspect,
		Provider:     provider,
		RequestID:    j.ID,
		Locale:       prompt.Extras.Locale,
		WatermarkTag: prompt.Watermark.Text,
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
