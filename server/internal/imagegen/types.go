package imagegen

import "context"

type SourceImage struct {
	URL      string
	Data     []byte
	MIMEType string
	Name     string
	Width    int
	Height   int
}

type GenerateRequest struct {
	Provider    string `json:"provider"`
	Quantity    int    `json:"quantity"`
	AspectRatio string `json:"aspect_ratio"`

	Prompt struct {
		Title        string `json:"title"`
		ProductType  string `json:"product_type"`
		Style        string `json:"style"`
		Background   string `json:"background"`
		Instructions string `json:"instructions"`
		Watermark    struct {
			Enabled  bool   `json:"enabled"`
			Text     string `json:"text"`
			Position string `json:"position"`
		} `json:"watermark"`
		References []struct {
			URL string `json:"url"`
		} `json:"references"`
		SourceAsset struct {
			AssetID string `json:"asset_id"`
			URL     string `json:"url"`
		} `json:"source_asset"`
		Extras map[string]any `json:"extras"`
	} `json:"prompt"`
}

type GenerateResponse struct {
	JobID   string   `json:"job_id"`
	Status  string   `json:"status"`
	Images  []string `json:"images,omitempty"`
	Message string   `json:"message,omitempty"`
}

type Editor interface {
	EditOnce(ctx context.Context, source SourceImage, instruction string, watermark bool, negative string, seed *int) (string, error)
}
