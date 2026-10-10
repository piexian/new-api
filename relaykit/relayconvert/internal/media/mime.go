package media

// FileMimeType 从文件名扩展名或 data URI 头推断 MIME 类型(供无法自带 mime 的入站媒体兜底)。

import (
	"path/filepath"
	"strings"

	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
)

func FileMimeType(file *kitdto.MessageFile) string {
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
