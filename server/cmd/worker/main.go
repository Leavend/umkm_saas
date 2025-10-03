package main

import (
	"context"
	"encoding/json"
	"time"

	"server/internal/domain/jsoncfg"
	"server/internal/infra"
	"server/internal/providers/image"
	videoprovider "server/internal/providers/video"
	"server/internal/sqlinline"

	"github.com/rs/zerolog"
)

func main() {
	cfg, err := infra.LoadConfig()
	if err != nil {
		panic(err)
	}
	logger := infra.NewLogger(cfg.AppEnv)

	ctx := context.Background()
	pool, err := infra.NewDBPool(ctx, cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("worker: db connection failed")
	}
	defer pool.Close()

	runner := infra.NewSQLRunner(pool, logger)
	imageProviders := map[string]image.Generator{
		"gemini":     image.NewNanoBanana(),
		"nanobanana": image.NewNanoBanana(),
	}
	videoProviders := map[string]videoprovider.Generator{
		"veo2": videoprovider.NewVEO(),
		"veo3": videoprovider.NewVEO(),
	}

	logger.Info().Msg("worker started")
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		var jobID, userID, taskType, provider string
		var quantity int
		var aspect string
		var promptBytes []byte
		row := runner.QueryRow(ctx, sqlinline.QWorkerClaimJob)
		if err := row.Scan(&jobID, &userID, &taskType, &provider, &quantity, &aspect, &promptBytes); err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		logger.Info().Str("job_id", jobID).Str("task_type", taskType).Msg("worker: picked job")
		status := "FAILED"
		switch taskType {
		case "IMAGE_GEN":
			status = processImageJob(ctx, runner, imageProviders, logger, jobID, userID, provider, quantity, aspect, promptBytes)
		case "VIDEO_GEN":
			status = processVideoJob(ctx, runner, videoProviders, logger, jobID, userID, provider, aspect, promptBytes)
		default:
			logger.Error().Str("job_id", jobID).Str("task_type", taskType).Msg("worker: unsupported job type")
		}
		if _, err := runner.Exec(ctx, sqlinline.QUpdateJobStatus, jobID, status); err != nil {
			logger.Error().Err(err).Msgf("worker: update status failed for %s", jobID)
		}
	}
}

func processImageJob(
	ctx context.Context,
	runner *infra.SQLRunner,
	providers map[string]image.Generator,
	logger zerolog.Logger,
	jobID, userID, provider string,
	quantity int,
	aspect string,
	promptBytes []byte,
) string {
	var prompt jsoncfg.PromptJSON
	_ = json.Unmarshal(promptBytes, &prompt)
	generator, ok := providers[provider]
	if !ok {
		provider = "gemini"
		generator = providers[provider]
	}
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
		logger.Error().Err(err).Str("job_id", jobID).Msg("worker: image generation failed")
		return "FAILED"
	}
	for _, asset := range assets {
		if _, execErr := runner.Exec(ctx, sqlinline.QInsertAsset, userID, "GENERATED", jobID, asset.URL, asset.Format, int64(1024*1024), asset.Width, asset.Height, aspect, jsoncfg.MustMarshal(map[string]any{"provider": provider})); execErr != nil {
			logger.Error().Err(execErr).Str("job_id", jobID).Msg("worker: insert image asset failed")
		}
	}
	return "SUCCEEDED"
}

func processVideoJob(
	ctx context.Context,
	runner *infra.SQLRunner,
	providers map[string]videoprovider.Generator,
	logger zerolog.Logger,
	jobID, userID, provider, aspect string,
	promptBytes []byte,
) string {
	payload := map[string]any{}
	_ = json.Unmarshal(promptBytes, &payload)
	promptText := extractPromptText(payload)
	locale := ""
	if v, ok := payload["locale"].(string); ok {
		locale = v
	}
	generator, ok := providers[provider]
	if !ok {
		provider = "veo2"
		generator = providers[provider]
	}
	asset, err := generator.Generate(ctx, videoprovider.GenerateRequest{
		Prompt:    promptText,
		Provider:  provider,
		RequestID: jobID,
		Locale:    locale,
	})
	if err != nil {
		logger.Error().Err(err).Str("job_id", jobID).Msg("worker: video generation failed")
		return "FAILED"
	}
	if _, execErr := runner.Exec(ctx, sqlinline.QInsertAsset, userID, "GENERATED", jobID, asset.URL, asset.Format, int64(5*1024*1024), 1920, 1080, aspect, jsoncfg.MustMarshal(map[string]any{"provider": provider, "length": asset.Length})); execErr != nil {
		logger.Error().Err(execErr).Str("job_id", jobID).Msg("worker: insert video asset failed")
	}
	return "SUCCEEDED"
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
