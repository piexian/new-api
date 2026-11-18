package hailuo

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/QuantumNous/new-api/constant"
	dto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	kitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
)

// https://platform.minimaxi.com/docs/api-reference/video-generation-intro
type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = miniMaxTaskRootBaseURL(info.ChannelBaseUrl)
	a.apiKey = info.ApiKey
}

func miniMaxTaskRootBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[constant.ChannelTypeMiniMax]
	}
	for _, suffix := range []string{"/v1", "/anthropic"} {
		baseURL = strings.TrimSuffix(baseURL, suffix)
	}
	return baseURL
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	endpoint := TextToVideoEndpoint
	if isH3VideoModel(info.UpstreamModelName) {
		endpoint = TextToVideoV2Endpoint
	}
	return fmt.Sprintf("%s%s", a.baseURL, endpoint), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req, ok := v.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil, fmt.Errorf("invalid request type in context")
	}

	var body any
	var err error
	if isH3VideoModel(info.UpstreamModelName) {
		body, err = a.convertToRequestPayloadV2(&req, info)
	} else {
		body, err = a.convertToRequestPayload(&req, info)
	}
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}

	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	if isH3VideoModel(info.UpstreamModelName) {
		var hRespV2 VideoResponseV2
		if err := common.Unmarshal(responseBody, &hRespV2); err != nil {
			taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
			return
		}
		if hRespV2.TaskID == "" {
			taskErr = service.TaskErrorWrapper(fmt.Errorf("hailuo v2 api error: empty task_id"), "empty_task_id", http.StatusBadRequest)
			return
		}

		ov := kitdto.NewOpenAIVideo()
		ov.ID = info.PublicTaskID
		ov.TaskID = info.PublicTaskID
		ov.CreatedAt = time.Now().Unix()
		ov.Model = info.OriginModelName

		c.JSON(http.StatusOK, ov)
		return hRespV2.TaskID, responseBody, nil
	}

	var hResp VideoResponse
	if err := common.Unmarshal(responseBody, &hResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	if hResp.BaseResp.StatusCode != StatusSuccess {
		taskErr = service.TaskErrorWrapper(
			fmt.Errorf("hailuo api error: %s", hResp.BaseResp.StatusMsg),
			strconv.Itoa(hResp.BaseResp.StatusCode),
			http.StatusBadRequest,
		)
		return
	}

	ov := kitdto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return hResp.TaskID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	var uri string
	if modelName, _ := body["model_name"].(string); isH3VideoModel(modelName) {
		// V2 查询走路径参数:GET /v2/query/video_generation/{task_id}
		uri = fmt.Sprintf("%s%s/%s", baseUrl, QueryTaskV2Endpoint, taskID)
	} else {
		uri = fmt.Sprintf("%s%s?task_id=%s", baseUrl, QueryTaskEndpoint, taskID)
	}

	req, err := http.NewRequest(http.MethodGet, uri, nil)
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

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) convertToRequestPayload(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*VideoRequest, error) {
	modelConfig := GetModelConfig(info.UpstreamModelName)
	duration := DefaultDuration
	if req.Duration > 0 {
		duration = req.Duration
	}
	resolution := modelConfig.DefaultResolution
	if req.Size != "" {
		resolution = a.parseResolutionFromSize(req.Size, modelConfig)
	}

	videoRequest := &VideoRequest{
		Model:      info.UpstreamModelName,
		Prompt:     req.Prompt,
		Duration:   &duration,
		Resolution: resolution,
	}
	a.applyImageInputs(req, videoRequest)
	if err := req.UnmarshalMetadata(&videoRequest); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata to video request failed")
	}

	return videoRequest, nil
}

func (a *TaskAdaptor) applyImageInputs(req *relaycommon.TaskSubmitReq, videoRequest *VideoRequest) {
	images := append([]string{}, req.Images...)
	if req.Image != "" && len(images) == 0 {
		images = append(images, req.Image)
	}
	if req.InputReference != "" && len(images) == 0 {
		images = append(images, req.InputReference)
	}
	if len(images) == 0 {
		return
	}

	mode := strings.ToLower(req.Mode)
	if videoRequest.Model == "S2V-01" || strings.Contains(mode, "subject") || strings.Contains(mode, "reference") || strings.Contains(mode, "s2v") {
		videoRequest.SubjectReference = []SubjectReference{{
			Type:  "character",
			Image: []string{images[0]},
		}}
		return
	}

	videoRequest.FirstFrameImage = images[0]
	if len(images) > 1 {
		videoRequest.LastFrameImage = images[1]
	}
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {

	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	if isH3VideoModel(info.UpstreamModelName) {
		return a.estimateH3Billing(&req, info)
	}
	videoRequest, err := a.convertToRequestPayload(&req, info)
	if err != nil {
		return nil
	}
	duration := DefaultDuration
	if videoRequest.Duration != nil && *videoRequest.Duration > 0 {
		duration = *videoRequest.Duration
	}
	if ratio := miniMaxVideoPriceRatio(videoRequest.Model, videoRequest.Resolution, duration); ratio > 0 && ratio != 1 {
		return map[string]float64{"duration_resolution": ratio}
	}
	return nil
}

func miniMaxVideoPriceRatio(model string, resolution string, duration int) float64 {
	key := fmt.Sprintf("%s:%s:%d", model, resolution, duration)
	ratios := map[string]float64{
		"MiniMax-Hailuo-2.3-Fast:768P:10": 2.25 / 1.35,
		"MiniMax-Hailuo-2.3-Fast:1080P:6": 2.31 / 1.35,
		"MiniMax-Hailuo-2.3:768P:10":      4.00 / 2.00,
		"MiniMax-Hailuo-2.3:1080P:6":      3.50 / 2.00,
		"MiniMax-Hailuo-02:768P:10":       4.00 / 2.00,
		"MiniMax-Hailuo-02:1080P:6":       3.50 / 2.00,
		"MiniMax-Hailuo-02:512P:6":        0.60 / 2.00,
		"MiniMax-Hailuo-02:512P:10":       1.00 / 2.00,
	}
	return ratios[key]
}

func (a *TaskAdaptor) parseResolutionFromSize(size string, modelConfig ModelConfig) string {
	switch {
	case strings.Contains(strings.ToLower(size), "2k"):
		return Resolution2K
	case strings.Contains(size, "1080"):
		return Resolution1080P
	case strings.Contains(size, "768"):
		return Resolution768P
	case strings.Contains(size, "720"):
		return Resolution720P
	case strings.Contains(size, "512"):
		return Resolution512P
	case strings.Contains(size, "480"):
		return Resolution480P
	default:
		return modelConfig.DefaultResolution
	}
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	// V2 响应无 base_resp 包裹,先按 task/flat 形状识别
	if v2Task, ok := parseVideoTaskV2FromResponse(respBody); ok {
		return parseVideoTaskV2Info(v2Task), nil
	}

	resTask := QueryTaskResponse{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{}

	if resTask.BaseResp.StatusCode == StatusSuccess {
		taskResult.Code = 0
	} else {
		taskResult.Code = resTask.BaseResp.StatusCode
		taskResult.Reason = resTask.BaseResp.StatusMsg
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
	}

	switch resTask.Status {
	case TaskStatusPreparing, TaskStatusQueueing, TaskStatusProcessing:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
		if resTask.Status == TaskStatusProcessing {
			taskResult.Progress = "50%"
		}
	case TaskStatusSuccess:
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = a.buildVideoURL(resTask.TaskID, resTask.FileID)
	case TaskStatusFailed:
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		if taskResult.Reason == "" {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var hailuoResp QueryTaskResponse
	if err := common.Unmarshal(originTask.Data, &hailuoResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal hailuo task data failed")
	}

	// V2 任务数据无 base_resp 包裹,错误信息取自 VideoTaskV2.Error
	if v2Task, ok := parseVideoTaskV2FromResponse(originTask.Data); ok {
		openAIVideo := originTask.ToOpenAIVideo()
		if v2Task.Error != nil && v2Task.Error.Message != "" {
			openAIVideo.Error = &kitdto.OpenAIVideoError{
				Message: v2Task.Error.Message,
				Code:    v2Task.Error.Code,
			}
		}
		jsonData, marshalErr := common.Marshal(openAIVideo)
		if marshalErr != nil {
			return nil, errors.Wrap(marshalErr, "marshal openai video failed")
		}
		return jsonData, nil
	}

	openAIVideo := originTask.ToOpenAIVideo()
	if hailuoResp.BaseResp.StatusCode != StatusSuccess {
		openAIVideo.Error = &kitdto.OpenAIVideoError{
			Message: hailuoResp.BaseResp.StatusMsg,
			Code:    strconv.Itoa(hailuoResp.BaseResp.StatusCode),
		}
	}

	jsonData, err := common.Marshal(openAIVideo)
	if err != nil {
		return nil, errors.Wrap(err, "marshal openai video failed")
	}

	return jsonData, nil
}

func (a *TaskAdaptor) buildVideoURL(_, fileID string) string {
	if a.apiKey == "" || a.baseURL == "" {
		return ""
	}

	url := fmt.Sprintf("%s/v1/files/retrieve?file_id=%s", a.baseURL, fileID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return ""
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := service.GetHttpClient().Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	var retrieveResp RetrieveFileResponse
	if err := common.Unmarshal(responseBody, &retrieveResp); err != nil {
		return ""
	}

	if retrieveResp.BaseResp.StatusCode != StatusSuccess {
		return ""
	}

	return retrieveResp.File.DownloadURL
}

// isH3VideoModel 判断模型是否走 V2 接口(H3 家族四款)
func isH3VideoModel(model string) bool {
	switch model {
	case "MiniMax-H3", "MiniMax-H3-Highspeed", "MiniMax-H3-Lite", "MiniMax-H3-Max":
		return true
	}
	return false
}

// convertToRequestPayloadV2 把网关任务请求转换为 H3 V2 请求:
// prompt→text 内容块,图片按 mode 映射 first/last_frame 或 reference_image,
// metadata 可直接覆盖/扩展 content、ratio、extra 等高级字段。
func (a *TaskAdaptor) convertToRequestPayloadV2(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*VideoRequestV2, error) {
	modelConfig := GetModelConfig(info.UpstreamModelName)
	duration := DefaultDuration
	if req.Duration > 0 {
		duration = req.Duration
	}
	resolution := modelConfig.DefaultResolution
	if req.Size != "" {
		resolution = a.parseResolutionFromSize(req.Size, modelConfig)
	}

	videoRequest := &VideoRequestV2{
		Model:      info.UpstreamModelName,
		Resolution: resolution,
		Duration:   duration,
		Content:    []ContentItemV2{{Type: "text", Text: req.Prompt}},
	}
	a.applyImageInputsV2(req, videoRequest)
	// 纯文生视频必须显式指定宽高比(adaptive 会被上游拒绝),未提供时缺省 16:9
	if len(videoRequest.Content) == 1 && videoRequest.Ratio == "" {
		videoRequest.Ratio = "16:9"
	}
	if err := req.UnmarshalMetadata(&videoRequest); err != nil {
		return nil, errors.Wrap(err, "unmarshal metadata to video request failed")
	}
	return videoRequest, nil
}

// applyImageInputsV2 把网关图片入参映射为 V2 content 图片块
func (a *TaskAdaptor) applyImageInputsV2(req *relaycommon.TaskSubmitReq, videoRequest *VideoRequestV2) {
	images := append([]string{}, req.Images...)
	if req.Image != "" && len(images) == 0 {
		images = append(images, req.Image)
	}
	if req.InputReference != "" && len(images) == 0 {
		images = append(images, req.InputReference)
	}
	if len(images) == 0 {
		return
	}

	mode := strings.ToLower(req.Mode)
	if strings.Contains(mode, "subject") || strings.Contains(mode, "reference") || strings.Contains(mode, "s2v") {
		for _, image := range images {
			videoRequest.Content = append(videoRequest.Content, ContentItemV2{
				Type:     "image_url",
				ImageURL: &MediaURLV2{URL: image},
				Role:     "reference_image",
			})
		}
		return
	}

	videoRequest.Content = append(videoRequest.Content, ContentItemV2{
		Type:     "image_url",
		ImageURL: &MediaURLV2{URL: images[0]},
		Role:     "first_frame",
	})
	if len(images) > 1 {
		videoRequest.Content = append(videoRequest.Content, ContentItemV2{
			Type:     "image_url",
			ImageURL: &MediaURLV2{URL: images[1]},
			Role:     "last_frame",
		})
	}
}

// estimateH3Billing 估算 H3 按秒计费乘数;输入视频时长提单时未知按 0 估,
// 任务完成时经 AdjustBillingOnComplete 按实际 usage 精确结算。
func (a *TaskAdaptor) estimateH3Billing(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) map[string]float64 {
	videoRequest, err := a.convertToRequestPayloadV2(req, info)
	if err != nil {
		return nil
	}
	duration := videoRequest.Duration
	if duration <= 0 {
		duration = DefaultDuration
	}
	imageCount := 0
	for _, item := range videoRequest.Content {
		if item.ImageURL != nil {
			imageCount++
		}
	}
	if ratio := miniMaxH3VideoRatio(videoRequest.Model, videoRequest.Resolution, duration, 0, imageCount); ratio > 0 {
		return map[string]float64{"duration_resolution": ratio}
	}
	return nil
}

// miniMaxH3VideoRatio 计算 H3 相对 768P 每秒基准价的用量乘数:
// 输出秒 × 分辨率系数 + 输入视频秒 × 输入系数 + 超免额图片张 × 图片系数;价格见官方定价页(2026-11)
func miniMaxH3VideoRatio(model, resolution string, outputSeconds, inputSeconds, imageCount int) float64 {
	var outScale, inScale, imgScale float64
	freeImages := 5
	switch model {
	case "MiniMax-H3":
		outScale = 1
		if resolution == Resolution2K {
			outScale = 0.80 / 0.50
		}
		inScale, imgScale = outScale, 0.20/0.50
	case "MiniMax-H3-Highspeed":
		outScale = 1
		if resolution == Resolution480P {
			outScale = 0.26 / 0.40
		}
		inScale, imgScale = outScale, 0.20/0.40
	case "MiniMax-H3-Lite":
		outScale = 1
		if resolution == Resolution480P {
			outScale = 0.16 / 0.26
		}
		inScale, imgScale = outScale, 0.13/0.26
	case "MiniMax-H3-Max":
		outScale = 1
		if resolution == Resolution480P {
			outScale = 0.33 / 0.50
			inScale = 0.37 / 0.50
		} else {
			inScale = 0.97 / 0.50
		}
		imgScale, freeImages = 1, 2
	default:
		return 0
	}
	extraImages := imageCount - freeImages
	if extraImages < 0 {
		extraImages = 0
	}
	ratio := float64(outputSeconds)*outScale + float64(inputSeconds)*inScale + float64(extraImages)*imgScale
	if ratio <= 0 {
		return 0
	}
	return ratio
}

// AdjustBillingOnComplete H3 任务成功时按 V2 usage 的实际秒数/图片张数精确结算差额
func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if !isH3VideoModel(task.Properties.UpstreamModelName) {
		return 0
	}
	v2Task, ok := parseVideoTaskV2FromResponse(task.Data)
	if !ok || v2Task.Status != "succeeded" || v2Task.Usage == nil {
		return 0
	}
	bc := task.PrivateData.BillingContext
	if bc == nil || bc.ModelRatio <= 0 {
		return 0
	}
	usage := v2Task.Usage
	ratio := miniMaxH3VideoRatio(v2Task.Model, v2Task.Resolution, usage.OutputSeconds, usage.InputSeconds, usage.InputImageCount)
	if ratio <= 0 {
		return 0
	}
	// 与 ModelPriceHelperPerCall 同口径:modelRatio/2 × QuotaPerUnit × groupRatio,再乘实际用量系数
	actualQuota, _ := common.QuotaFromFloatChecked(bc.ModelRatio / 2 * common.QuotaPerUnit * bc.GroupRatio * ratio)
	return actualQuota
}

// parseVideoTaskV2FromResponse 识别 V2 查询响应(V1 响应恒带 base_resp)
func parseVideoTaskV2FromResponse(respBody []byte) (*VideoTaskV2, bool) {
	var envelope struct {
		BaseResp *BaseResp    `json:"base_resp"`
		Task     *VideoTaskV2 `json:"task"`
	}
	if err := common.Unmarshal(respBody, &envelope); err != nil {
		return nil, false
	}
	if envelope.BaseResp != nil {
		return nil, false
	}
	if envelope.Task != nil {
		return envelope.Task, true
	}
	var flat VideoTaskV2
	if err := common.Unmarshal(respBody, &flat); err == nil && flat.ID != "" && flat.Status != "" {
		return &flat, true
	}
	return nil, false
}

// parseVideoTaskV2Info 把 V2 任务状态映射为站内 TaskInfo;产物为限时直链,无需 files/retrieve
func parseVideoTaskV2Info(task *VideoTaskV2) *relaycommon.TaskInfo {
	taskResult := relaycommon.TaskInfo{
		TaskID:   task.ID,
		Status:   model.TaskStatusInProgress,
		Progress: "30%",
	}
	switch task.Status {
	case "running":
		taskResult.Progress = "50%"
	case "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		if task.Content != nil {
			taskResult.Url = task.Content.URL
			taskResult.RemoteUrl = task.Content.URL
		}
	case "failed", "cancelled":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = "task failed"
		if task.Error != nil && task.Error.Message != "" {
			taskResult.Reason = task.Error.Message
		}
	}
	return &taskResult
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsInt(slice []int, item int) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
