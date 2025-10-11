package zip

import (
	"archive/zip"
	"bytes"
)

type Asset struct {
	Filename string
	MIME     string
	Data     []byte
}

func ArchiveAssets(assets []Asset) []byte {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for _, asset := range assets {
		w, err := zw.Create(asset.Filename)
		if err != nil {
			continue
		}
		if _, err := w.Write(asset.Data); err != nil {
			return nil
		}
	}
	_ = zw.Close()
	return buf.Bytes()
}
