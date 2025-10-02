package domain

import "time"

// AssetKind enumerates asset types.
type AssetKind string

const (
	AssetKindImage AssetKind = "image"
	AssetKindVideo AssetKind = "video"
)

// Asset represents a generated artifact belonging to a job.
type Asset struct {
	ID        string
	JobID     string
	Kind      AssetKind
	URL       string
	Width     int
	Height    int
	Checksum  string
	Bytes     int64
	CreatedAt time.Time
}

// GeneratedAsset is used by providers to return produced assets prior to persistence.
type GeneratedAsset struct {
	Kind     AssetKind
	URL      string
	Width    int
	Height   int
	Bytes    int64
	Checksum string
	Metadata map[string]any
}

// InputAsset describes an input asset for enhancement workflows.
type InputAsset struct {
	Kind string
	URL  string
	Data []byte
}
