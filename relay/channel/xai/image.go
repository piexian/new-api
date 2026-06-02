package xai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
)

func convertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	imageReq := ImageRequest{
		Model:          request.Model,
		Prompt:         request.Prompt,
		N:              request.N,
		ResponseFormat: request.ResponseFormat,
		User:           request.User,
		Image:          request.Image,
		Images:         request.Images,
	}

	if imageReq.Model == "" && info != nil {
		imageReq.Model = info.UpstreamModelName
	}

	rawFields := make(map[string]json.RawMessage)
	for key, raw := range request.Extra {
		rawFields[key] = raw
	}
	mergeRawFields(rawFields, request.ExtraFields)

	if aspectRatio := rawString(rawFields["aspect_ratio"]); aspectRatio != "" {
		imageReq.AspectRatio = aspectRatio
	} else if aspectRatio := imageAspectRatioFromSize(request.Size); aspectRatio != "" {
		imageReq.AspectRatio = aspectRatio
	}
	if resolution := rawString(rawFields["resolution"]); resolution != "" {
		imageReq.Resolution = resolution
	}
	if imageReq.ResponseFormat == "" {
		if imageFormat := rawString(rawFields["image_format"]); imageFormat != "" {
			imageReq.ResponseFormat = normalizeXAIImageFormat(imageFormat)
		}
	}
	if normalized, ok := normalizeXAIImageRaw(imageReq.Image); ok {
		imageReq.Image = normalized
	}
	if normalized, ok := normalizeXAIImagesRaw(imageReq.Images); ok {
		imageReq.Images = normalized
	}
	if raw, ok := rawFields["image"]; ok && len(imageReq.Image) == 0 {
		imageReq.Image = normalizeXAIImageRawOrOriginal(raw)
	}
	if raw, ok := rawFields["image_url"]; ok && len(imageReq.Image) == 0 {
		imageReq.Image = normalizeXAIImageRawOrOriginal(raw)
	}
	if raw, ok := rawFields["images"]; ok && len(imageReq.Images) == 0 {
		imageReq.Images = normalizeXAIImagesRawOrOriginal(raw)
	}
	if raw, ok := rawFields["image_urls"]; ok && len(imageReq.Images) == 0 {
		imageReq.Images = normalizeXAIImagesRawOrOriginal(raw)
	}

	if info != nil && info.RelayMode == relayconstant.RelayModeImagesEdits && isMultipartRequest(c) {
		images, err := imageSourcesFromMultipart(c)
		if err != nil {
			return nil, err
		}
		if len(images) == 1 {
			imageReq.Image = mustMarshalRaw(images[0])
		} else {
			imageReq.Images = mustMarshalRaw(images)
		}
	}

	result := structToRawMap(imageReq)
	for key, raw := range rawFields {
		if shouldSkipImageExtra(key) {
			continue
		}
		result[key] = raw
	}
	return result, nil
}

func BuildImageLogDetails(request dto.ImageRequest, convertedRequest any, info *relaycommon.RelayInfo) map[string]interface{} {
	payload := mapFromAny(convertedRequest)
	if len(payload) == 0 {
		payload = imageRequestLogPayload(request, info)
	}
	if len(payload) == 0 {
		return nil
	}
	return BuildGenerationLogDetails("image", payload, false)
}

func imageRequestLogPayload(request dto.ImageRequest, info *relaycommon.RelayInfo) map[string]any {
	data, err := common.Marshal(request)
	if err != nil {
		return nil
	}
	payload := make(map[string]any)
	_ = common.Unmarshal(data, &payload)
	for key, raw := range request.Extra {
		if value, ok := rawLogValue(raw); ok {
			payload[key] = value
		}
	}
	extraFields := make(map[string]json.RawMessage)
	mergeRawFields(extraFields, request.ExtraFields)
	for key, raw := range extraFields {
		if _, exists := payload[key]; exists {
			continue
		}
		if value, ok := rawLogValue(raw); ok {
			payload[key] = value
		}
	}
	if model := strings.TrimSpace(request.Model); model != "" {
		payload["model"] = model
	} else if info != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		payload["model"] = strings.TrimSpace(info.UpstreamModelName)
	}
	if _, ok := payload["aspect_ratio"]; !ok {
		if aspectRatio := imageAspectRatioFromSize(request.Size); aspectRatio != "" {
			payload["aspect_ratio"] = aspectRatio
		}
	}
	if _, ok := payload["response_format"]; !ok {
		if imageFormat := rawString(request.Extra["image_format"]); imageFormat != "" {
			payload["response_format"] = normalizeXAIImageFormat(imageFormat)
		}
	}
	if _, ok := payload["image"]; !ok {
		if image, ok := payload["image_url"]; ok {
			payload["image"] = imageLogSourceValue(image)
		}
	}
	if _, ok := payload["images"]; !ok {
		if images, ok := payload["image_urls"]; ok {
			payload["images"] = imageLogSourcesValue(images)
		}
	}
	return payload
}

func mergeRawFields(fields map[string]json.RawMessage, extraFields json.RawMessage) {
	if len(extraFields) == 0 {
		return
	}
	var extra map[string]json.RawMessage
	if err := common.Unmarshal(extraFields, &extra); err != nil {
		return
	}
	for key, raw := range extra {
		if _, exists := fields[key]; !exists {
			fields[key] = raw
		}
	}
}

func rawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func rawLogValue(raw json.RawMessage) (any, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false
	}
	var value any
	if err := common.Unmarshal(raw, &value); err != nil {
		return nil, false
	}
	return value, true
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case map[string]json.RawMessage:
		out := make(map[string]any, len(typed))
		for key, raw := range typed {
			if value, ok := rawLogValue(raw); ok {
				out[key] = value
			}
		}
		return out
	default:
		data, err := common.Marshal(value)
		if err != nil {
			return nil
		}
		out := make(map[string]any)
		if err := common.Unmarshal(data, &out); err != nil {
			return nil
		}
		return out
	}
}

func imageLogSourceValue(value any) any {
	if url, ok := value.(string); ok && strings.TrimSpace(url) != "" {
		return map[string]any{
			"type": "image_url",
			"url":  strings.TrimSpace(url),
		}
	}
	return value
}

func imageLogSourcesValue(value any) any {
	switch typed := value.(type) {
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, imageLogSourceValue(item))
		}
		return out
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, imageLogSourceValue(item))
		}
		return out
	default:
		return value
	}
}

func BuildGenerationLogDetails(kind string, payload map[string]any, includePrompt bool) map[string]interface{} {
	if len(payload) == 0 {
		return nil
	}
	details := map[string]interface{}{
		"provider": "xai",
		"type":     kind,
	}
	addLogDetail := func(key string) {
		if key == "prompt" && !includePrompt {
			return
		}
		value, ok := payload[key]
		if !ok {
			return
		}
		value = sanitizeGenerationLogValue(value)
		if !isDisplayableGenerationLogValue(value) {
			return
		}
		details[key] = value
	}

	for _, key := range []string{
		"model",
		"prompt",
		"size",
		"quality",
		"aspect_ratio",
		"resolution",
		"duration",
		"n",
		"response_format",
		"background",
		"moderation",
		"partial_images",
		"watermark",
		"seed",
		"negative_prompt",
		"style_preset",
		"image",
		"images",
		"reference_images",
		"video",
	} {
		addLogDetail(key)
	}

	known := map[string]struct{}{
		"provider": {}, "type": {}, "model": {}, "prompt": {}, "size": {}, "quality": {},
		"aspect_ratio": {}, "resolution": {}, "duration": {}, "n": {}, "response_format": {},
		"background": {}, "moderation": {}, "partial_images": {}, "watermark": {}, "seed": {},
		"negative_prompt": {}, "style_preset": {}, "image": {}, "images": {},
		"reference_images": {}, "video": {}, "image_url": {}, "image_urls": {},
		"reference_image_urls": {}, "image_format": {},
	}
	for key, value := range payload {
		if _, ok := known[key]; ok {
			continue
		}
		value = sanitizeGenerationLogValue(value)
		if !isDisplayableGenerationLogValue(value) {
			continue
		}
		details[key] = value
	}

	if len(details) <= 2 {
		return nil
	}
	return details
}

func sanitizeGenerationLogValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return sanitizeGenerationLogString(typed)
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if sanitized := sanitizeGenerationLogValue(item); isDisplayableGenerationLogValue(sanitized) {
				out[key] = sanitized
			}
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if sanitized := sanitizeGenerationLogValue(item); isDisplayableGenerationLogValue(sanitized) {
				out[key] = sanitized
			}
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			if sanitized := sanitizeGenerationLogValue(item); isDisplayableGenerationLogValue(sanitized) {
				out = append(out, sanitized)
			}
		}
		return out
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			if sanitized := sanitizeGenerationLogValue(item); isDisplayableGenerationLogValue(sanitized) {
				out = append(out, sanitized)
			}
		}
		return out
	case json.RawMessage:
		if value, ok := rawLogValue(typed); ok {
			return sanitizeGenerationLogValue(value)
		}
		return nil
	default:
		return typed
	}
}

func sanitizeGenerationLogString(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "data:") {
		if comma := strings.Index(value, ","); comma >= 0 {
			return value[:comma+1] + "<omitted>"
		}
		return "data:<omitted>"
	}
	const maxLogStringLength = 512
	if len(value) > maxLogStringLength {
		return value[:maxLogStringLength] + "..."
	}
	return value
}

func isDisplayableGenerationLogValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case map[string]any:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	default:
		return true
	}
}

func shouldSkipImageExtra(key string) bool {
	switch key {
	case "model", "prompt", "n", "response_format", "user", "aspect_ratio", "resolution", "image", "images", "image_url", "image_urls", "image_format":
		return true
	default:
		return false
	}
}

func normalizeXAIImageFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "base64":
		return "b64_json"
	default:
		return strings.TrimSpace(format)
	}
}

func normalizeXAIImageRawOrOriginal(raw json.RawMessage) json.RawMessage {
	if normalized, ok := normalizeXAIImageRaw(raw); ok {
		return normalized
	}
	return raw
}

func normalizeXAIImagesRawOrOriginal(raw json.RawMessage) json.RawMessage {
	if normalized, ok := normalizeXAIImagesRaw(raw); ok {
		return normalized
	}
	return raw
}

func normalizeXAIImageRaw(raw json.RawMessage) (json.RawMessage, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false
	}
	if url := rawString(raw); url != "" {
		return mustMarshalRaw(newImageURLSource(url)), true
	}
	var object map[string]any
	if err := common.Unmarshal(raw, &object); err == nil && len(object) > 0 {
		if url, ok := object["url"].(string); ok && strings.TrimSpace(url) != "" {
			if _, hasType := object["type"]; !hasType {
				object["type"] = "image_url"
			}
			return mustMarshalRaw(object), true
		}
		if fileID, ok := object["file_id"].(string); ok && strings.TrimSpace(fileID) != "" {
			return mustMarshalRaw(object), true
		}
	}
	return nil, false
}

func normalizeXAIImagesRaw(raw json.RawMessage) (json.RawMessage, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false
	}
	var items []json.RawMessage
	if err := common.Unmarshal(raw, &items); err != nil || len(items) == 0 {
		if normalized, ok := normalizeXAIImageRaw(raw); ok {
			return mustMarshalRaw([]json.RawMessage{normalized}), true
		}
		return nil, false
	}

	sources := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		normalized, ok := normalizeXAIImageRaw(item)
		if !ok {
			return nil, false
		}
		sources = append(sources, normalized)
	}
	return mustMarshalRaw(sources), true
}

func structToRawMap(v any) map[string]json.RawMessage {
	data, _ := common.Marshal(v)
	out := make(map[string]json.RawMessage)
	_ = common.Unmarshal(data, &out)
	return out
}

func mustMarshalRaw(v any) json.RawMessage {
	data, _ := common.Marshal(v)
	return data
}

func isMultipartRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	return strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data")
}

func imageSourcesFromMultipart(c *gin.Context) ([]xAIImageSource, error) {
	form, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image edit form request: %w", err)
	}

	imageFiles := collectImageFileHeaders(form)
	if len(imageFiles) == 0 {
		if urls := collectImageURLs(form); len(urls) > 0 {
			return urls, nil
		}
		return nil, errors.New("image is required")
	}
	if len(imageFiles) > 3 {
		return nil, errors.New("xAI image edits support up to 3 source images")
	}

	sources := make([]xAIImageSource, 0, len(imageFiles))
	for _, fh := range imageFiles {
		source, err := imageFileHeaderToSource(fh)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, nil
}

func collectImageFileHeaders(form *multipart.Form) []*multipart.FileHeader {
	if form == nil || form.File == nil {
		return nil
	}
	var files []*multipart.FileHeader
	seen := make(map[*multipart.FileHeader]struct{})
	appendFiles := func(values []*multipart.FileHeader) {
		for _, value := range values {
			if value == nil {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			files = append(files, value)
		}
	}
	for _, key := range []string{"image", "image[]", "images"} {
		if values := form.File[key]; len(values) > 0 {
			appendFiles(values)
		}
	}
	for fieldName, values := range form.File {
		if strings.HasPrefix(fieldName, "image[") && len(values) > 0 {
			appendFiles(values)
		}
	}
	return files
}

func collectImageURLs(form *multipart.Form) []xAIImageSource {
	if form == nil || form.Value == nil {
		return nil
	}
	var urls []xAIImageSource
	seen := make(map[string]struct{})
	appendURL := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		urls = append(urls, newImageURLSource(value))
	}
	for _, key := range []string{"image", "image[]", "images", "image_url", "image_urls"} {
		for _, value := range form.Value[key] {
			appendURL(value)
		}
	}
	for fieldName, values := range form.Value {
		if strings.HasPrefix(fieldName, "image[") || strings.HasPrefix(fieldName, "images[") || strings.HasPrefix(fieldName, "image_urls[") {
			for _, value := range values {
				appendURL(value)
			}
		}
	}
	return urls
}

func imageFileHeaderToSource(fh *multipart.FileHeader) (xAIImageSource, error) {
	file, err := fh.Open()
	if err != nil {
		return xAIImageSource{}, fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return xAIImageSource{}, fmt.Errorf("failed to read image file: %w", err)
	}
	mimeType := fh.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(data)
	}
	return newImageURLSource(fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))), nil
}

func newImageURLSource(url string) xAIImageSource {
	return xAIImageSource{
		Type: "image_url",
		URL:  url,
	}
}

func imageAspectRatioFromSize(size string) string {
	switch strings.TrimSpace(size) {
	case "1024x1024", "1536x1536":
		return "1:1"
	case "1792x1024", "1536x864", "1280x720":
		return "16:9"
	case "1024x1792", "864x1536", "720x1280":
		return "9:16"
	case "1536x1024", "1248x832":
		return "3:2"
	case "1024x1536", "832x1248":
		return "2:3"
	case "1152x864":
		return "4:3"
	case "864x1152":
		return "3:4"
	case "1344x576":
		return "21:9"
	default:
		return ""
	}
}

func (a *Adaptor) convertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	a.ResponseFormat = request.ResponseFormat
	switch info.RelayMode {
	case relayconstant.RelayModeAudioSpeech:
		body, err := convertTTSRequest(request)
		if err != nil {
			return nil, err
		}
		data, err := common.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("error marshalling object: %w", err)
		}
		return bytes.NewReader(data), nil
	case relayconstant.RelayModeAudioTranscription, relayconstant.RelayModeAudioTranslation:
		return passthroughAudioMultipart(c, request)
	default:
		return nil, errors.New("not available")
	}
}

func convertTTSRequest(request dto.AudioRequest) (map[string]any, error) {
	body := map[string]any{
		"text":     request.Input,
		"voice_id": request.Voice,
	}
	if body["voice_id"] == "" {
		delete(body, "voice_id")
	}
	body["language"] = "auto"

	if request.ResponseFormat != "" {
		body["output_format"] = map[string]any{
			"codec": request.ResponseFormat,
		}
	}
	if request.Speed != nil {
		body["speed"] = *request.Speed
	}
	mergeAudioMetadata(body, request.Metadata)
	if body["text"] == "" {
		return nil, errors.New("input is required")
	}
	return body, nil
}

func mergeAudioMetadata(body map[string]any, metadata json.RawMessage) {
	if len(metadata) == 0 {
		return
	}
	var fields map[string]any
	if err := common.Unmarshal(metadata, &fields); err != nil {
		return
	}
	for key, value := range fields {
		if key == "" {
			continue
		}
		body[key] = value
	}
}

func passthroughAudioMultipart(c *gin.Context, request dto.AudioRequest) (io.Reader, error) {
	form, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return nil, fmt.Errorf("error parsing multipart form: %w", err)
	}

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	if request.Model != "" {
		_ = writer.WriteField("model", request.Model)
	}
	for key, values := range form.Value {
		if key == "model" {
			continue
		}
		for _, value := range values {
			_ = writer.WriteField(key, value)
		}
	}
	for fieldName, fileHeaders := range form.File {
		for _, fileHeader := range fileHeaders {
			if err := writeMultipartFile(writer, fieldName, fileHeader); err != nil {
				_ = writer.Close()
				return nil, err
			}
		}
	}
	_ = writer.Close()
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return &requestBody, nil
}

func writeMultipartFile(writer *multipart.Writer, fieldName string, fileHeader *multipart.FileHeader) error {
	file, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("error opening file %s: %w", fieldName, err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile(fieldName, fileHeader.Filename)
	if err != nil {
		return fmt.Errorf("create form file failed: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("copy file failed: %w", err)
	}
	return nil
}
