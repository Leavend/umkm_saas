package main

import (
	"context"
	"encoding/json"
	"time"

	"server/internal/domain/jsoncfg"
	"server/internal/infra"
	"server/internal/providers/image"
	"server/internal/sqlinline"
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
	providers := map[string]image.Generator{
		"gemini":     image.NewNanoBanana(),
		"nanobanana": image.NewNanoBanana(),
	}

	logger.Info().Msg("worker started")
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		var jobID, userID, provider string
		var quantity int
		var aspect string
		var promptBytes []byte
		row := runner.QueryRow(ctx, sqlinline.QWorkerClaimJob)
		if err := row.Scan(&jobID, &userID, &provider, &quantity, &aspect, &promptBytes); err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		logger.Info().Msgf("worker: picked job %s", jobID)
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
		status := "SUCCEEDED"
		if err != nil {
			status = "FAILED"
			logger.Error().Err(err).Msgf("worker: generation failed for %s", jobID)
		} else {
			for _, asset := range assets {
				_, execErr := runner.Exec(ctx, sqlinline.QInsertAsset, userID, "GENERATED", jobID, asset.URL, asset.Format, int64(1024*1024), asset.Width, asset.Height, aspect, jsoncfg.MustMarshal(map[string]any{"provider": provider}))
				if execErr != nil {
					logger.Error().Err(execErr).Msgf("worker: insert asset failed for %s", jobID)
				}
			}
		}
		if _, err := runner.Exec(ctx, sqlinline.QUpdateJobStatus, jobID, status); err != nil {
			logger.Error().Err(err).Msgf("worker: update status failed for %s", jobID)
		}
	}
}
