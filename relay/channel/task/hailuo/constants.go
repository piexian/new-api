package hailuo

const (
	ChannelName = "hailuo-video"
)

var ModelList = []string{
	// H3 家族走 V2 接口(POST /v2/video_generation)
	"MiniMax-H3",
	"MiniMax-H3-Highspeed",
	"MiniMax-H3-Lite",
	"MiniMax-H3-Max",
	// Hailuo 走 V1 接口
	"MiniMax-Hailuo-2.3",
	"MiniMax-Hailuo-2.3-Fast",
	"MiniMax-Hailuo-02",
}

const (
	TextToVideoEndpoint = "/v1/video_generation"
	QueryTaskEndpoint   = "/v1/query/video_generation"

	TextToVideoV2Endpoint = "/v2/video_generation"
	QueryTaskV2Endpoint   = "/v2/query/video_generation"
)

const (
	StatusSuccess    = 0
	StatusRateLimit  = 1002
	StatusAuthFailed = 1004
	StatusNoBalance  = 1008
	StatusSensitive  = 1026
	StatusParamError = 2013
	StatusInvalidKey = 2049
)

const (
	TaskStatusPreparing  = "Preparing"
	TaskStatusQueueing   = "Queueing"
	TaskStatusProcessing = "Processing"
	TaskStatusSuccess    = "Success"
	TaskStatusFailed     = "Fail"
)

const (
	Resolution512P  = "512P"
	Resolution480P  = "480P"
	Resolution720P  = "720P"
	Resolution768P  = "768P"
	Resolution1080P = "1080P"
	Resolution2K    = "2K"
)

const (
	DefaultDuration   = 6
	DefaultResolution = Resolution720P
)
