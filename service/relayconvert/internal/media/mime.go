package media

import (
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

func FileMimeType(file *dto.MessageFile) string {
	if strings.HasPrefix(file.FileData, "data:") {
		if comma := strings.Index(file.FileData, ","); comma >= 0 {
			header := strings.SplitN(file.FileData[len("data:"):comma], ";", 2)
			return strings.ToLower(header[0])
		}
	}
	switch strings.ToLower(filepath.Ext(file.FileName)) {
	case ".pdf":
		return "application/pdf"
	case ".txt", ".md", ".markdown", ".csv", ".log":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".xml":
		return "application/xml"
	case ".html", ".htm":
		return "text/html"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return ""
	}
}
