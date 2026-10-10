package types

// relaykit 迁移：请求元数据已统一到 relaykit/types，此处保留别名以维持宿主引用。
import (
	kittypes "github.com/QuantumNous/new-api/relaykit/types"
)

type FileType = kittypes.FileType

const (
	FileTypeImage = kittypes.FileTypeImage
	FileTypeFile  = kittypes.FileTypeFile
	FileTypeVideo = kittypes.FileTypeVideo
	FileTypeAudio = kittypes.FileTypeAudio
)

type TokenType = kittypes.TokenType

const (
	TokenTypeTextNumber = kittypes.TokenTypeTextNumber
	TokenTypeTokenizer  = kittypes.TokenTypeTokenizer
	TokenTypeImage      = kittypes.TokenTypeImage
)

type TokenCountMeta = kittypes.TokenCountMeta

type FileMeta = kittypes.FileMeta

func NewFileMeta(fileType FileType, source FileSource) *FileMeta {
	return kittypes.NewFileMeta(fileType, source)
}

func NewImageFileMeta(source FileSource, detail string) *FileMeta {
	return kittypes.NewImageFileMeta(source, detail)
}

type RequestMeta = kittypes.RequestMeta
