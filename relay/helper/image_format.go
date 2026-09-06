package helper

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// Cerebras 图片能力: 仅 gemma 系列(Dedicated Endpoint)支持图片输入,
// 且只收 PNG/JPEG 的 base64 data URI, 外部 URL 图片同样会 400。
// gpt-oss-120b / qwen-3.8-27b 等公共端点模型为纯文本模型, 任何图片都会 400。
// 参考 https://inference-docs.cerebras.ai/capabilities/image-inputs.md
const (
	cerebrasVisionModelPrefix = "gemma"
	cerebrasImageMimePNG      = "image/png"
	cerebrasImageMimeJPEG     = "image/jpeg"
	cerebrasImageMimeJPG      = "image/jpg"
)

// imageExtensionMimeTypes 只用于从 URL 扩展名推断图片类型, 推断不出时按"未知"处理
var imageExtensionMimeTypes = map[string]string{
	"png":  "image/png",
	"jpeg": "image/jpeg",
	"jpg":  "image/jpg",
	"jfif": "image/jpeg",
	"gif":  "image/gif",
	"webp": "image/webp",
	"bmp":  "image/bmp",
	"tiff": "image/tiff",
	"tif":  "image/tiff",
	"heic": "image/heic",
	"heif": "image/heif",
	"avif": "image/avif",
}

// mimeUnknown 表示请求体里没有足够信息判定图片类型(例如无扩展名的远程 URL)
const mimeUnknown = "unknown"

// requestImage 描述请求中的一个输入图片
type requestImage struct {
	mimeType string
	// isRemoteURL 表示图片以 http(s) URL 形式提供, 而非 base64 data URI
	isRemoteURL bool
}

// ValidateCerebrasImageInput 在出站前拦截 Cerebras 渠道上必然失败的图片请求:
//   - 纯文本模型(gpt-oss-120b / qwen-3.8-27b 等)携带任何图片
//   - gemma 视觉模型携带非 PNG/JPEG 图片, 或携带外部 URL 图片
//
// 判定只依赖请求体本身(显式声明 / data URI 前缀 / URL 扩展名), 不做网络探测。
func ValidateCerebrasImageInput(c *gin.Context, info *common.RelayInfo, request dto.Request) *types.NewAPIError {
	if info == nil || info.ChannelMeta == nil || request == nil {
		return nil
	}
	if info.ChannelMeta.ChannelType != constant.ChannelTypeCerebras {
		return nil
	}
	modelName := info.UpstreamModelName
	if modelName == "" {
		modelName = info.OriginModelName
	}
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return nil
	}

	images := requestImages(request)
	if len(images) == 0 {
		return nil
	}

	if !isCerebrasVisionModel(modelName) {
		return cerebrasImageError(
			fmt.Sprintf("model %q does not support image input", modelName),
		)
	}

	for _, image := range images {
		if image.isRemoteURL {
			return cerebrasImageError(
				fmt.Sprintf("model %q only accepts base64-encoded image data, external image URLs are not supported", modelName),
			)
		}
		switch image.mimeType {
		case cerebrasImageMimePNG, cerebrasImageMimeJPEG, cerebrasImageMimeJPG:
			continue
		case mimeUnknown:
			// 无法从请求体判定类型时放行, 交给上游判定, 避免误杀
			continue
		default:
			return cerebrasImageError(
				fmt.Sprintf("model %q only accepts PNG and JPEG images, got %q", modelName, image.mimeType),
			)
		}
	}
	return nil
}

func isCerebrasVisionModel(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(modelName), cerebrasVisionModelPrefix)
}

func cerebrasImageError(message string) *types.NewAPIError {
	return types.NewOpenAIError(
		fmt.Errorf("%s", message),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

// requestImages 提取请求中所有输入图片
func requestImages(request dto.Request) []requestImage {
	switch req := request.(type) {
	case *dto.GeneralOpenAIRequest:
		var images []requestImage
		for i := range req.Messages {
			images = append(images, messageImages(req.Messages[i])...)
		}
		return images
	case *dto.ClaudeRequest:
		var images []requestImage
		for i := range req.Messages {
			parts, err := req.Messages[i].ParseContent()
			if err != nil {
				continue
			}
			for _, part := range parts {
				if part.Source == nil {
					continue
				}
				data, _ := part.Source.Data.(string)
				if data != "" {
					images = append(images, imageFromSource(data, part.Source.MediaType))
				}
				if part.Source.Url != "" {
					images = append(images, imageFromSource(part.Source.Url, part.Source.MediaType))
				}
			}
		}
		return images
	case *dto.GeminiChatRequest:
		var images []requestImage
		contents := req.Contents
		if len(contents) == 0 {
			for i := range req.Requests {
				contents = append(contents, req.Requests[i].Contents...)
			}
		}
		for i := range contents {
			for j := range contents[i].Parts {
				part := &contents[i].Parts[j]
				if part.InlineData != nil {
					images = append(images, imageFromSource("", part.InlineData.MimeType))
				}
				if part.FileData != nil {
					images = append(images, imageFromSource(part.FileData.FileUri, part.FileData.MimeType))
				}
			}
		}
		return images
	case *dto.OpenAIResponsesRequest:
		var images []requestImage
		for _, input := range req.ParseInput() {
			if input.Type != "input_image" {
				continue
			}
			images = append(images, imageFromSource(input.ImageUrl, ""))
		}
		return images
	case *dto.GeminiInteractionsRequest:
		return interactionImages(req.Input)
	}
	return nil
}

func messageImages(message dto.Message) []requestImage {
	if message.Content == nil {
		return nil
	}
	var parts []dto.MediaContent
	if typed, ok := message.Content.([]dto.MediaContent); ok {
		parts = typed
	} else {
		parts = message.ParseContent()
	}
	var images []requestImage
	for i := range parts {
		if parts[i].Type != dto.ContentTypeImageURL {
			continue
		}
		image := parts[i].GetImageMedia()
		if image == nil {
			continue
		}
		images = append(images, imageFromSource(image.Url, image.MimeType))
	}
	return images
}

// interactionImages 扫描 interactions input 中的图片内容
func interactionImages(input []byte) []requestImage {
	if len(input) == 0 {
		return nil
	}
	var images []requestImage
	collectInteractionImages(gjson.ParseBytes(input), &images)
	return images
}

func collectInteractionImages(result gjson.Result, images *[]requestImage) {
	switch {
	case result.IsArray():
		for _, item := range result.Array() {
			collectInteractionImages(item, images)
		}
	case result.IsObject():
		if mime := result.Get("mime_type"); mime.Exists() {
			data := result.Get("data").String()
			uri := result.Get("uri").String()
			source := data
			if source == "" {
				source = uri
			}
			*images = append(*images, imageFromSource(source, mime.String()))
		}
		result.ForEach(func(_, value gjson.Result) bool {
			collectInteractionImages(value, images)
			return true
		})
	}
}

// imageFromSource 判定图片类型, 优先级: 显式声明 > data URI 前缀 > URL 扩展名推断
func imageFromSource(raw string, declared string) requestImage {
	if mimeType := normalizeMime(declared); mimeType != "" {
		return requestImage{mimeType: mimeType, isRemoteURL: isRemoteURL(raw)}
	}
	if idx := strings.Index(raw, ","); strings.HasPrefix(raw, "data:") && idx != -1 {
		mimeType := normalizeMime(raw[len("data:"):idx])
		if mimeType == "" {
			mimeType = mimeUnknown
		}
		return requestImage{mimeType: mimeType}
	}
	if isRemoteURL(raw) {
		if mimeType := urlExtensionImageMime(raw); mimeType != "" {
			return requestImage{mimeType: mimeType, isRemoteURL: true}
		}
		return requestImage{mimeType: mimeUnknown, isRemoteURL: true}
	}
	// 裸 base64 且无声明类型, 无法判定
	return requestImage{mimeType: mimeUnknown}
}

func normalizeMime(mimeType string) string {
	mimeType = strings.TrimSpace(strings.ToLower(mimeType))
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	return mimeType
}

func isRemoteURL(raw string) bool {
	return strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://")
}

func urlExtensionImageMime(rawURL string) string {
	if idx := strings.IndexAny(rawURL, "?#"); idx != -1 {
		rawURL = rawURL[:idx]
	}
	segment := rawURL
	if idx := strings.LastIndex(segment, "/"); idx != -1 {
		segment = segment[idx+1:]
	}
	dot := strings.LastIndex(segment, ".")
	if dot == -1 || dot+1 >= len(segment) {
		return ""
	}
	return imageExtensionMimeTypes[strings.ToLower(segment[dot+1:])]
}
