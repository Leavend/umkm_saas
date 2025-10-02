package zip

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
)

type Asset struct {
	Filename string
	MIME     string
	URL      string
}

func ArchiveAssets(assets []Asset) []byte {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for _, asset := range assets {
		w, err := zw.Create(asset.Filename)
		if err != nil {
			continue
		}
		if _, err := io.WriteString(w, asset.URL); err != nil {
			return nil, fmt.Errorf("write url: %w", err)
		}
	}
	_ = zw.Close()
	return buf.Bytes()
}
