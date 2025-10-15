package imagegen

import (
	"fmt"
	"strings"
)

func BuildInstruction(req GenerateRequest) string {
	parts := []string{}
	title := strings.TrimSpace(req.Prompt.Title)
	productType := strings.TrimSpace(req.Prompt.ProductType)
	switch {
	case title != "" && productType != "":
		parts = append(parts, fmt.Sprintf("Edit foto produk agar tampil sebagai \"%s\" (jenis: %s).", title, productType))
	case title != "":
		parts = append(parts, fmt.Sprintf("Edit foto produk agar tampil sebagai \"%s\".", title))
	case productType != "":
		parts = append(parts, fmt.Sprintf("Edit foto produk agar menonjolkan jenis %s.", productType))
	}
	if style := strings.TrimSpace(req.Prompt.Style); style != "" {
		parts = append(parts, "Gaya visual: "+style+".")
	}
	if background := strings.TrimSpace(req.Prompt.Background); background != "" {
		parts = append(parts, "Ganti/atur latar: "+background+".")
	}
	if instructions := strings.TrimSpace(req.Prompt.Instructions); instructions != "" {
		parts = append(parts, "Instruksi tambahan: "+instructions+".")
	}
	parts = append(parts, "Pertahankan bentuk produk asli, proporsi natural, tidak blur, tidak cacat.")
	if aspect := strings.TrimSpace(req.AspectRatio); aspect != "" {
		parts = append(parts, "Komposisi menyesuaikan rasio "+aspect+".")
	}
	if len(req.Prompt.References) > 1 {
		for idx, ref := range req.Prompt.References {
			url := strings.TrimSpace(ref.URL)
			if url == "" {
				continue
			}
			parts = append(parts, fmt.Sprintf("Gunakan referensi gambar %d: %s", idx+1, url))
		}
	}
	return strings.Join(parts, " ")
}
