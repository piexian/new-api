package types

// relaykit 迁移：文件来源抽象已统一到 relaykit/types，此处保留别名以维持宿主引用。
import (
	kittypes "github.com/QuantumNous/new-api/relaykit/types"
)

type FileSource = kittypes.FileSource

type URLSource = kittypes.URLSource

type Base64Source = kittypes.Base64Source

type CachedFileData = kittypes.CachedFileData

func NewURLFileSource(url string) *URLSource { return kittypes.NewURLFileSource(url) }

func NewBase64FileSource(base64Data string, mimeType string) *Base64Source {
	return kittypes.NewBase64FileSource(base64Data, mimeType)
}

func NewFileSourceFromData(data string, mimeType string) FileSource {
	return kittypes.NewFileSourceFromData(data, mimeType)
}

func NewMemoryCachedData(base64Data string, mimeType string, size int64) *CachedFileData {
	return kittypes.NewMemoryCachedData(base64Data, mimeType, size)
}

func NewDiskCachedData(diskPath string, mimeType string, size int64) *CachedFileData {
	return kittypes.NewDiskCachedData(diskPath, mimeType, size)
}
