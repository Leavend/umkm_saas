package domain

// PromptContract represents the JSON prompt contract for generation requests.
type PromptContract struct {
	Title        string            `json:"title" validate:"required,min=3,max=120"`
	ProductType  string            `json:"product_type" validate:"required,oneof=food fashion skincare shoes bag other"`
	Style        string            `json:"style" validate:"required,oneof=elegan minimalis luxury fun custom"`
	Background   string            `json:"background" validate:"required,oneof=studio_white solid_color marble wood fabric gradient cafe kitchen minimal_room outdoor_picnic custom"`
	Instructions string            `json:"instructions" validate:"max=500"`
	Watermark    WatermarkConfig   `json:"watermark" validate:"required,dive"`
	AspectRatio  string            `json:"aspect_ratio" validate:"required,oneof=1:1 4:3 3:4 16:9 9:16"`
	Quantity     int               `json:"quantity" validate:"required,min=1,max=10"`
	References   []PromptReference `json:"references" validate:"dive"`
	Extras       PromptExtras      `json:"extras" validate:"required,dive"`
}

// WatermarkConfig configures the overlay watermark on assets.
type WatermarkConfig struct {
	Enabled  bool   `json:"enabled"`
	Text     string `json:"text" validate:"required_if=Enabled true"`
	Position string `json:"position" validate:"required_if=Enabled true,oneof=top-left top-right bottom-left bottom-right"`
}

// PromptReference contains optional base64 or URL reference data.
type PromptReference struct {
	Type       string `json:"type" validate:"required,oneof=image"`
	URL        string `json:"url" validate:"omitempty,url"`
	DataBase64 string `json:"data_base64" validate:"omitempty,base64"`
}

// PromptExtras contains optional extras metadata.
type PromptExtras struct {
	Locale  string `json:"locale" validate:"required,oneof=id en"`
	Quality string `json:"quality" validate:"required,oneof=standard hd ultra"`
}
