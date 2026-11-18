package hailuo

type SubjectReference struct {
	Type  string   `json:"type"`  // Subject type, currently only supports "character"
	Image []string `json:"image"` // Array of subject reference images (currently only supports single image)
}

type VideoRequest struct {
	Model            string             `json:"model"`
	Prompt           string             `json:"prompt,omitempty"`
	PromptOptimizer  *bool              `json:"prompt_optimizer,omitempty"`
	FastPretreatment *bool              `json:"fast_pretreatment,omitempty"`
	Duration         *int               `json:"duration,omitempty"`
	Resolution       string             `json:"resolution,omitempty"`
	CallbackURL      string             `json:"callback_url,omitempty"`
	AigcWatermark    *bool              `json:"aigc_watermark,omitempty"`
	FirstFrameImage  string             `json:"first_frame_image,omitempty"` // For image-to-video and start-end-to-video
	LastFrameImage   string             `json:"last_frame_image,omitempty"`  // For start-end-to-video
	SubjectReference []SubjectReference `json:"subject_reference,omitempty"` // For subject-reference-to-video
}

type VideoResponse struct {
	TaskID   string   `json:"task_id"`
	BaseResp BaseResp `json:"base_resp"`
}

type BaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

type QueryTaskRequest struct {
	TaskID string `json:"task_id"`
}

type QueryTaskResponse struct {
	TaskID      string   `json:"task_id"`
	Status      string   `json:"status"`
	FileID      string   `json:"file_id,omitempty"`
	VideoWidth  int      `json:"video_width,omitempty"`
	VideoHeight int      `json:"video_height,omitempty"`
	BaseResp    BaseResp `json:"base_resp"`
}

type ErrorInfo struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

type TaskStatusInfo struct {
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	FileID    string `json:"file_id,omitempty"`
	VideoURL  string `json:"video_url,omitempty"`
	ErrorCode int    `json:"error_code,omitempty"`
	ErrorMsg  string `json:"error_msg,omitempty"`
}

type ModelConfig struct {
	Name                 string
	DefaultResolution    string
	SupportedDurations   []int
	SupportedResolutions []string
	HasPromptOptimizer   bool
	HasFastPretreatment  bool
}

type RetrieveFileResponse struct {
	File     FileObject `json:"file"`
	BaseResp BaseResp   `json:"base_resp"`
}

type FileObject struct {
	FileID      int64  `json:"file_id"`
	Bytes       int64  `json:"bytes"`
	CreatedAt   int64  `json:"created_at"`
	Filename    string `json:"filename"`
	Purpose     string `json:"purpose"`
	DownloadURL string `json:"download_url"`
}

// ── V2 接口(H3 家族)类型 ──────────────────────────────────────────

// ContentItemV2 是 V2 请求的 content 数组元素;text 必填,媒体经 role 标注用途
type ContentItemV2 struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	ImageURL *MediaURLV2 `json:"image_url,omitempty"`
	VideoURL *MediaURLV2 `json:"video_url,omitempty"`
	AudioURL *MediaURLV2 `json:"audio_url,omitempty"`
	Role     string      `json:"role,omitempty"`
}

// MediaURLV2 对齐 V2 媒体对象:公网 URL / mm_file:// / data: URI
type MediaURLV2 struct {
	URL string `json:"url"`
}

// VideoRequestV2 是 POST /v2/video_generation 的请求体
type VideoRequestV2 struct {
	Model         string          `json:"model"`
	Content       []ContentItemV2 `json:"content"`
	Resolution    string          `json:"resolution"`
	Duration      int             `json:"duration"`
	Ratio         string          `json:"ratio,omitempty"`
	CallbackURL   string          `json:"callback_url,omitempty"`
	AigcWatermark *bool           `json:"aigc_watermark,omitempty"`
	Extra         *VideoExtraV2   `json:"extra,omitempty"`
}

// VideoExtraV2 仅 H3-Max 支持 prompt_expansion_mode
type VideoExtraV2 struct {
	PromptExpansionMode string `json:"prompt_expansion_mode,omitempty"`
}

// VideoResponseV2 是 V2 提交响应:{task_id};失败为 OpenAI 错误体
type VideoResponseV2 struct {
	TaskID string `json:"task_id"`
}

// VideoTaskV2 是 V2 查询响应中的任务对象(task 包裹或 flat)
type VideoTaskV2 struct {
	ID         string              `json:"id"`
	Model      string              `json:"model"`
	Status     string              `json:"status"`
	Error      *VideoTaskErrorV2   `json:"error,omitempty"`
	CreatedAt  int64               `json:"created_at,omitempty"`
	UpdatedAt  int64               `json:"updated_at,omitempty"`
	Content    *VideoTaskContentV2 `json:"content,omitempty"`
	Resolution string              `json:"resolution,omitempty"`
	Duration   int                 `json:"duration,omitempty"`
	Usage      *VideoTaskUsageV2   `json:"usage,omitempty"`
	Ratio      string              `json:"ratio,omitempty"`
	TaskType   string              `json:"task_type,omitempty"`
	Modality   string              `json:"modality,omitempty"`
}

// VideoTaskErrorV2 任务失败时的错误信息
type VideoTaskErrorV2 struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// VideoTaskContentV2 成功任务产物:视频 url 或 Context-IR prompt
type VideoTaskContentV2 struct {
	URL    string `json:"url,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

// VideoTaskUsageV2 视频任务按秒/张计量,Context-IR 按 token
type VideoTaskUsageV2 struct {
	TotalSeconds      int `json:"total_seconds,omitempty"`
	InputSeconds      int `json:"input_seconds,omitempty"`
	OutputSeconds     int `json:"output_seconds,omitempty"`
	InputImageCount   int `json:"input_image_count,omitempty"`
	InputAudioSeconds int `json:"input_audio_seconds,omitempty"`
	TotalTokens       int `json:"total_tokens,omitempty"`
	PromptTokens      int `json:"prompt_tokens,omitempty"`
	CompletionTokens  int `json:"completion_tokens,omitempty"`
}

// GetVideoTaskV2Response 是 response_format=task 的查询响应
type GetVideoTaskV2Response struct {
	Task *VideoTaskV2 `json:"task"`
}

func GetModelConfig(model string) ModelConfig {
	configs := map[string]ModelConfig{
		"MiniMax-Hailuo-2.3": {
			Name:                 "MiniMax-Hailuo-2.3",
			DefaultResolution:    Resolution768P,
			SupportedDurations:   []int{6, 10},
			SupportedResolutions: []string{Resolution768P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  true,
		},
		"MiniMax-Hailuo-2.3-Fast": {
			Name:                 "MiniMax-Hailuo-2.3-Fast",
			DefaultResolution:    Resolution768P,
			SupportedDurations:   []int{6, 10},
			SupportedResolutions: []string{Resolution768P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  true,
		},
		"MiniMax-Hailuo-02": {
			Name:                 "MiniMax-Hailuo-02",
			DefaultResolution:    Resolution768P,
			SupportedDurations:   []int{6, 10},
			SupportedResolutions: []string{Resolution512P, Resolution768P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  true,
		},
		"T2V-01-Director": {
			Name:                 "T2V-01-Director",
			DefaultResolution:    Resolution768P,
			SupportedDurations:   []int{6},
			SupportedResolutions: []string{Resolution768P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  false,
		},
		"T2V-01": {
			Name:                 "T2V-01",
			DefaultResolution:    Resolution720P,
			SupportedDurations:   []int{6},
			SupportedResolutions: []string{Resolution720P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  false,
		},
		"I2V-01-Director": {
			Name:                 "I2V-01-Director",
			DefaultResolution:    Resolution720P,
			SupportedDurations:   []int{6},
			SupportedResolutions: []string{Resolution720P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  false,
		},
		"I2V-01-live": {
			Name:                 "I2V-01-live",
			DefaultResolution:    Resolution720P,
			SupportedDurations:   []int{6},
			SupportedResolutions: []string{Resolution720P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  false,
		},
		"I2V-01": {
			Name:                 "I2V-01",
			DefaultResolution:    Resolution720P,
			SupportedDurations:   []int{6},
			SupportedResolutions: []string{Resolution720P, Resolution1080P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  false,
		},
		"S2V-01": {
			Name:                 "S2V-01",
			DefaultResolution:    Resolution720P,
			SupportedDurations:   []int{6},
			SupportedResolutions: []string{Resolution720P},
			HasPromptOptimizer:   true,
			HasFastPretreatment:  false,
		},
		"MiniMax-H3": {
			Name:                 "MiniMax-H3",
			DefaultResolution:    Resolution768P,
			SupportedDurations:   []int{4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			SupportedResolutions: []string{Resolution480P, Resolution768P, Resolution2K},
		},
		"MiniMax-H3-Highspeed": {
			Name:                 "MiniMax-H3-Highspeed",
			DefaultResolution:    Resolution768P,
			SupportedDurations:   []int{4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			SupportedResolutions: []string{Resolution480P, Resolution768P},
		},
		"MiniMax-H3-Lite": {
			Name:                 "MiniMax-H3-Lite",
			DefaultResolution:    Resolution768P,
			SupportedDurations:   []int{4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			SupportedResolutions: []string{Resolution480P, Resolution768P},
		},
		"MiniMax-H3-Max": {
			Name:                 "MiniMax-H3-Max",
			DefaultResolution:    Resolution768P,
			SupportedDurations:   []int{5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			SupportedResolutions: []string{Resolution480P, Resolution768P},
		},
	}

	if config, exists := configs[model]; exists {
		return config
	}

	return ModelConfig{
		Name:                 model,
		DefaultResolution:    DefaultResolution,
		SupportedDurations:   []int{6},
		SupportedResolutions: []string{DefaultResolution},
		HasPromptOptimizer:   true,
		HasFastPretreatment:  false,
	}
}
