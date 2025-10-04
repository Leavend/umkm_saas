-- +goose Up
ALTER TABLE usage_events DROP CONSTRAINT IF EXISTS usage_events_event_type_check;
ALTER TABLE usage_events
    ADD CONSTRAINT usage_events_event_type_check
    CHECK (event_type IN ('IMAGE_GEN','VIDEO_GEN','UPSCALE','PROMPT_ENHANCE','PROMPT_RANDOM','PROMPT_CLEAR'));

-- +goose Down
ALTER TABLE usage_events DROP CONSTRAINT IF EXISTS usage_events_event_type_check;
ALTER TABLE usage_events
    ADD CONSTRAINT usage_events_event_type_check
    CHECK (event_type IN ('IMAGE_GEN','VIDEO_GEN','UPSCALE'));
