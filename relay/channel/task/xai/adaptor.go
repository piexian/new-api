package xai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	imagexai "github.com/QuantumNous/new-api/relay/channel/xai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

const (
	actionVideoGenerate = appconstant.TaskActionGenerate
	actionVideoEdit     = "videoEdit"
	actionVideoExtend   = "videoExtend"

	pathVideoGenerations = "/v1/videos/generations"
	pathVideoEdits       = "/v1/videos/edits"
	pathVideoExtensions  = "/v1/videos/extensions"

	xaiVideoModelBasic        = "grok-imagine-video"
	xaiVideoModelPreview      = "grok-imagine-video-1.5-preview"
	xaiVideoModelPreviewAlias = "grok-imagine-video-1.5-2026-05-30"

	maxXAIReferenceImages = 7
)

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

type videoSubmitResponse struct {
	RequestID string `json:"request_id"`
}

type videoError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type videoResultResponse struct {
	Status   string `json:"status"`
	Progress *int   `json:"progress,omitempty"`
	Model    string `json:"model,omitempty"`
	Video    *struct {
		URL               string          `json:"url,omitempty"`
		Duration          json.RawMessage `json:"duration,omitempty"`
		RespectModeration *bool           `json:"respect_moderation,omitempty"`
	} `json:"video,omitempty"`
	Error *videoError `json:"error,omitempty"`
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}
	if err := applyMultipartImageInputs(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Model) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model field is required"), "missing_model", http.StatusBadRequest)
	}
	if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		req.Images = []string{req.Image}
	}

	info.Action = resolveAction(c, req)
	if isXAIImageRequiredVideoModel(req.Model) && info.Action == actionVideoGenerate && !hasXAIImageInput(req) {
		return service.TaskErrorWrapperLocal(fmt.Errorf("image is required for %s", req.Model), "invalid_request", http.StatusBadRequest)
	}
	c.Set("task_request", req)
	return nil
}

func resolveAction(c *gin.Context, req relaycommon.TaskSubmitReq) string {
	path := c.Request.URL.Path
	switch {
	case strings.HasPrefix(path, pathVideoEdits):
		return actionVideoEdit
	case strings.HasPrefix(path, pathVideoExtensions):
		return actionVideoExtend
	}
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	switch mode {
	case "edit", "edit-video", "video_edit":
		return actionVideoEdit
	case "extend", "extend-video", "extension", "video_extend":
		return actionVideoExtend
	default:
		return actionVideoGenerate
	}
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	seconds := req.Duration
	if seconds <= 0 {
		seconds = common.String2Int(req.Seconds)
	}
	if seconds <= 0 {
		return nil
	}
	return map[string]float64{"seconds": float64(seconds)}
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if !isXAIVideoModel(info.UpstreamModelName) {
		return "", fmt.Errorf("xAI video task model %q must be a video model", info.UpstreamModelName)
	}
	switch info.Action {
	case actionVideoEdit:
		return a.baseURL + pathVideoEdits, nil
	case actionVideoExtend:
		return a.baseURL + pathVideoExtensions, nil
	default:
		return a.baseURL + pathVideoGenerations, nil
	}
}

func isXAIVideoModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case xaiVideoModelBasic, xaiVideoModelPreview, xaiVideoModelPreviewAlias:
		return true
	default:
		return false
	}
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	body, err := convertToRequestPayload(req, info)
	if err != nil {
		return nil, err
	}
	if details := imagexai.BuildGenerationLogDetails("video", body, true); len(details) > 0 {
		c.Set(service.GenerationParamsContextKey, details)
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func convertToRequestPayload(req relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (map[string]any, error) {
	body := make(map[string]any)
	for key, value := range req.Metadata {
		if key == "model" {
			continue
		}
		body[key] = value
	}

	body["model"] = info.UpstreamModelName
	body["prompt"] = req.Prompt
	if req.Duration > 0 {
		body["duration"] = req.Duration
	} else if req.Seconds != "" {
		if seconds := common.String2Int(req.Seconds); seconds > 0 {
			body["duration"] = seconds
		}
	}

	if _, ok := body["aspect_ratio"]; !ok && req.Size != "" {
		if aspectRatio := aspectRatioFromSize(req.Size); aspectRatio != "" {
			body["aspect_ratio"] = aspectRatio
		}
	}

	switch info.Action {
	case actionVideoEdit, actionVideoExtend:
		if refs, _, err := xaiReferenceImagesFromBody(body); err != nil {
			return nil, err
		} else if len(refs) > 0 {
			return nil, fmt.Errorf("reference_images cannot be combined with xAI video edits/extensions")
		}
		if video, ok := body["video"]; ok {
			if url := mediaSourceURL(video); url != "" {
				body["video"] = map[string]any{"url": url}
			}
		} else {
			video := firstNonEmpty(metadataString(req.Metadata, "video_url"), metadataString(req.Metadata, "video"), req.InputReference, req.Image)
			if video == "" && len(req.Images) > 0 {
				video = req.Images[0]
			}
			if video == "" {
				return nil, fmt.Errorf("video is required for xAI video edits/extensions")
			}
			body["video"] = map[string]any{"url": video}
		}
	default:
		refs, fromBody, err := normalizeXAIReferenceImages(body, req)
		if err != nil {
			return nil, err
		}
		if len(refs) > 0 {
			if xaiHasReferenceModeConflict(body, req, fromBody) {
				return nil, fmt.Errorf("reference_images cannot be combined with image-to-video or video editing inputs")
			}
			applyXAIReferenceImages(body, refs)
			if duration, ok := xaiVideoDuration(body["duration"]); ok && duration > 10 {
				return nil, fmt.Errorf("duration cannot exceed 10 seconds when using reference_images")
			}
		} else {
			requireImage := isXAIImageRequiredVideoModel(info.UpstreamModelName)
			imageURL := normalizeXAIImageInput(body, req)
			if requireImage && imageURL == "" {
				return nil, fmt.Errorf("image is required for %s", info.UpstreamModelName)
			}
		}
	}
	return body, nil
}

func isXAIImageRequiredVideoModel(model string) bool {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case xaiVideoModelPreview, xaiVideoModelPreviewAlias:
		return true
	default:
		return false
	}
}

func hasXAIImageInput(req relaycommon.TaskSubmitReq) bool {
	if xaiImageInputURL(req) != "" {
		return true
	}
	if len(xaiReferenceImagesFromStrings(req.Images)) > 1 {
		return true
	}
	if refs, ok, _ := xaiReferenceImagesFromValue(req.Metadata["reference_images"]); ok && len(refs) > 0 {
		return true
	}
	if refs, ok, _ := xaiReferenceImagesFromValue(req.Metadata["reference_image_urls"]); ok && len(refs) > 0 {
		return true
	}
	return false
}

func applyMultipartImageInputs(c *gin.Context, req *relaycommon.TaskSubmitReq) error {
	if c == nil || c.Request == nil || !strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
		return nil
	}
	form, err := common.ParseMultipartFormReusable(c)
	if err != nil {
		return fmt.Errorf("failed to parse xAI video multipart form: %w", err)
	}
	files := xaiTaskImageFileHeaders(form)
	if len(files) == 0 {
		return nil
	}

	images := make([]string, 0, len(files))
	for _, fh := range files {
		image, err := xaiTaskImageFileToDataURI(fh)
		if err != nil {
			return err
		}
		images = append(images, image)
	}
	if len(images) == 1 {
		if xaiImageInputURL(*req) == "" {
			req.Image = images[0]
		}
		if len(req.Images) == 0 {
			req.Images = []string{images[0]}
		}
		return nil
	}
	if len(req.Images) == 0 {
		req.Images = images
	}
	return nil
}

func xaiTaskImageFileHeaders(form *multipart.Form) []*multipart.FileHeader {
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
	for _, key := range []string{"image", "image[]", "images", "input_reference", "reference_images", "reference_image_urls"} {
		if values := form.File[key]; len(values) > 0 {
			appendFiles(values)
		}
	}
	for fieldName, values := range form.File {
		if (strings.HasPrefix(fieldName, "image[") || strings.HasPrefix(fieldName, "images[") || strings.HasPrefix(fieldName, "reference_images[")) && len(values) > 0 {
			appendFiles(values)
		}
	}
	return files
}

func xaiTaskImageFileToDataURI(fh *multipart.FileHeader) (string, error) {
	file, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read image file: %w", err)
	}
	mimeType := fh.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(data)
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data)), nil
}

func normalizeXAIImageInput(body map[string]any, req relaycommon.TaskSubmitReq) string {
	if image, ok := body["image"]; ok {
		if url := mediaSourceURL(image); url != "" {
			body["image"] = map[string]any{"url": url}
			delete(body, "image_url")
			return url
		}
	}
	url := xaiImageInputURL(req)
	if url == "" {
		return ""
	}
	body["image"] = map[string]any{"url": url}
	delete(body, "image_url")
	return url
}

func xaiImageInputURL(req relaycommon.TaskSubmitReq) string {
	if url := mediaSourceURL(req.Metadata["image"]); url != "" {
		return url
	}
	if url := mediaSourceURL(req.Metadata["image_url"]); url != "" {
		return url
	}
	if image := strings.TrimSpace(req.Image); image != "" {
		return image
	}
	if ref := strings.TrimSpace(req.InputReference); ref != "" {
		return ref
	}
	if len(req.Images) == 1 {
		for _, image := range req.Images {
			if image := strings.TrimSpace(image); image != "" {
				return image
			}
		}
	}
	return ""
}

func normalizeXAIReferenceImages(body map[string]any, req relaycommon.TaskSubmitReq) ([]map[string]any, bool, error) {
	if refs, ok, err := xaiReferenceImagesFromBody(body); ok || err != nil {
		if err != nil {
			return nil, false, err
		}
		return refs, true, nil
	}

	refs := xaiReferenceImagesFromStrings(req.Images)
	if len(refs) <= 1 {
		return nil, false, nil
	}
	if len(refs) > maxXAIReferenceImages {
		return nil, false, fmt.Errorf("reference_images cannot contain more than %d images", maxXAIReferenceImages)
	}
	return refs, false, nil
}

func xaiReferenceImagesFromBody(body map[string]any) ([]map[string]any, bool, error) {
	if refs, ok, err := xaiReferenceImagesFromValue(body["reference_images"]); ok || err != nil {
		return refs, ok, err
	}
	return xaiReferenceImagesFromValue(body["reference_image_urls"])
}

func applyXAIReferenceImages(body map[string]any, refs []map[string]any) {
	body["reference_images"] = refs
	delete(body, "reference_image_urls")
	delete(body, "image")
	delete(body, "image_url")
}

func xaiReferenceImagesFromValue(value any) ([]map[string]any, bool, error) {
	if value == nil {
		return nil, false, nil
	}

	var items []any
	switch typed := value.(type) {
	case []any:
		items = typed
	case []string:
		items = make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, item)
		}
	case []map[string]any:
		items = make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, item)
		}
	default:
		return nil, true, fmt.Errorf("reference_images must be an array")
	}

	refs := make([]map[string]any, 0, len(items))
	for _, item := range items {
		ref, ok := xaiReferenceImageFromValue(item)
		if !ok {
			return nil, true, fmt.Errorf("reference_images items must include a non-empty url")
		}
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		return nil, true, fmt.Errorf("reference_images must contain at least one image")
	}
	if len(refs) > maxXAIReferenceImages {
		return nil, true, fmt.Errorf("reference_images cannot contain more than %d images", maxXAIReferenceImages)
	}
	return refs, true, nil
}

func xaiReferenceImageFromValue(value any) (map[string]any, bool) {
	if url, ok := value.(string); ok {
		url = strings.TrimSpace(url)
		if url == "" {
			return nil, false
		}
		return map[string]any{"url": url}, true
	}
	if data, ok := value.(map[string]any); ok {
		url := mediaSourceURL(data)
		if url == "" {
			return nil, false
		}
		ref := make(map[string]any, len(data))
		for key, item := range data {
			ref[key] = item
		}
		ref["url"] = url
		return ref, true
	}
	if data, ok := value.(map[string]string); ok {
		url := mediaSourceURL(data)
		if url == "" {
			return nil, false
		}
		ref := make(map[string]any, len(data))
		for key, item := range data {
			ref[key] = item
		}
		ref["url"] = url
		return ref, true
	}
	return nil, false
}

func xaiReferenceImagesFromStrings(images []string) []map[string]any {
	refs := make([]map[string]any, 0, len(images))
	for _, image := range images {
		image = strings.TrimSpace(image)
		if image != "" {
			refs = append(refs, map[string]any{"url": image})
		}
	}
	return refs
}

func xaiHasReferenceModeConflict(body map[string]any, req relaycommon.TaskSubmitReq, fromBody bool) bool {
	if mediaSourceURL(body["image"]) != "" || mediaSourceURL(body["image_url"]) != "" {
		return true
	}
	if mediaSourceURL(body["video"]) != "" || mediaSourceURL(body["video_url"]) != "" {
		return true
	}
	if strings.TrimSpace(req.Image) != "" || strings.TrimSpace(req.InputReference) != "" {
		return true
	}
	if fromBody && len(req.Images) > 0 {
		return true
	}
	return false
}

func xaiVideoDuration(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		duration := common.String2Int(typed)
		return duration, duration > 0
	default:
		return 0, false
	}
}

func mediaSourceURL(value any) string {
	if value == nil {
		return ""
	}
	if url, ok := value.(string); ok {
		return strings.TrimSpace(url)
	}
	if data, ok := value.(map[string]any); ok {
		if url, ok := data["url"].(string); ok {
			return strings.TrimSpace(url)
		}
	}
	if data, ok := value.(map[string]string); ok {
		return strings.TrimSpace(data["url"])
	}
	return ""
}

func metadataString(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	if value, ok := metadata[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func aspectRatioFromSize(size string) string {
	switch strings.TrimSpace(size) {
	case "1280x720", "1792x1024", "1536x864":
		return "16:9"
	case "720x1280", "1024x1792", "864x1536":
		return "9:16"
	case "1024x1024", "720x720":
		return "1:1"
	case "1152x864":
		return "4:3"
	case "864x1152":
		return "3:4"
	case "1536x1024", "1248x832":
		return "3:2"
	case "1024x1536", "832x1248":
		return "2:3"
	default:
		return ""
	}
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	var submitResp videoSubmitResponse
	if err := common.Unmarshal(responseBody, &submitResp); err != nil {
		return "", nil, service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	if strings.TrimSpace(submitResp.RequestID) == "" {
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("request_id is empty"), "invalid_response", http.StatusInternalServerError)
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)
	return submitResp.RequestID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}

	url := fmt.Sprintf("%s/v1/videos/%s", strings.TrimRight(baseUrl, "/"), taskID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var result videoResultResponse
	if err := common.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal xai video result failed: %w", err)
	}

	taskInfo := &relaycommon.TaskInfo{}
	switch strings.ToLower(strings.TrimSpace(result.Status)) {
	case "pending", "processing", "queued", "running", "":
		taskInfo.Status = model.TaskStatusInProgress
		taskInfo.Progress = progressString(result.Progress, "30%")
	case "done":
		taskInfo.Status = model.TaskStatusSuccess
		taskInfo.Progress = progressString(result.Progress, "100%")
		if result.Video != nil {
			taskInfo.Url = result.Video.URL
		}
	case "failed", "expired":
		taskInfo.Status = model.TaskStatusFailure
		taskInfo.Progress = progressString(result.Progress, "100%")
		if result.Error != nil {
			taskInfo.Reason = firstNonEmpty(result.Error.Message, result.Error.Code, result.Status)
		} else {
			taskInfo.Reason = result.Status
		}
	default:
		return nil, fmt.Errorf("unknown xai video status: %s", result.Status)
	}
	return taskInfo, nil
}

func progressString(progress *int, fallback string) string {
	if progress == nil {
		return fallback
	}
	value := *progress
	if value < 0 {
		value = 0
	} else if value > 100 {
		value = 100
	}
	return fmt.Sprintf("%d%%", value)
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var result videoResultResponse
	_ = common.Unmarshal(originTask.Data, &result)

	openAIVideo := originTask.ToOpenAIVideo()
	openAIVideo.TaskID = originTask.TaskID
	if result.Model != "" {
		openAIVideo.Model = result.Model
	}
	if result.Video != nil {
		if result.Video.URL != "" {
			openAIVideo.SetMetadata("url", result.Video.URL)
		}
		if len(result.Video.Duration) > 0 {
			openAIVideo.SetMetadata("duration", result.Video.Duration)
		}
		if result.Video.RespectModeration != nil {
			openAIVideo.SetMetadata("respect_moderation", *result.Video.RespectModeration)
		}
	}
	if result.Error != nil {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: result.Error.Message,
			Code:    result.Error.Code,
		}
	}
	return common.Marshal(openAIVideo)
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{xaiVideoModelBasic, xaiVideoModelPreview, xaiVideoModelPreviewAlias}
}

func (a *TaskAdaptor) GetChannelName() string {
	return "xai"
}
