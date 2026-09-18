package constant

// StepFun（阶跃星辰）渠道常量与协议能力快照。
//
// 上游有两条互不通用的通道（2026-09-19 实测）：
//   - 开放平台：https://api.stepfun.com(/v1)、https://api.stepfun.ai(/v1)
//   - Step Plan：{开放平台域名}/step_plan(/v1)，订阅制，仅文本三端点 + TTS + ASR-SSE + 音色 + WS
//
// base 白名单别名（渠道 base_url 可填别名，由 relay/channel/stepfun 解析）：
//   - stepfun                  国内开放平台
//   - stepfun-intl             国际开放平台
//   - stepfun-step-plan        国内 Step Plan
//   - stepfun-intl-step-plan   国际 Step Plan
const (
	StepFunBaseURLAlias             = "stepfun"
	StepFunIntlBaseURLAlias         = "stepfun-intl"
	StepFunStepPlanBaseURLAlias     = "stepfun-step-plan"
	StepFunIntlStepPlanBaseURLAlias = "stepfun-intl-step-plan"

	StepFunRootCNBaseURL   = "https://api.stepfun.com"
	StepFunRootIntlBaseURL = "https://api.stepfun.ai"
	StepFunStepPlanPath    = "/step_plan"

	// StepFunNativeFallbackModel 为无 model 字段的原生端点（Files、音色列表等）兜底路由模型。
	// 与 Moark 的 moark-task 同构：随渠道模型清单一并预填，便于渠道路由命中。
	StepFunNativeFallbackModel = "stepfun-native"
	// StepFunDataURLPrefix 供下游媒体入参为 data URL 时识别（base64 直传）。
	StepFunDataURLPrefix = "data:"
)

// 协议能力快照：来源为上游 GET /v1/models/{id} 的 supported_protocols（2026-09-19 实测）。
// 仅收录「确认受限」的模型集合，未收录模型不做限制（代理/自定义 base 场景不误伤）。
var (
	// StepFunResponsesUnsupportedModels 不支持 /v1/responses。
	StepFunResponsesUnsupportedModels = []string{
		"step-1o-turbo-vision",
		"step-3.5-flash",
		"step-3.5-flash-2603",
	}
	// StepFunMessagesUnsupportedModels 不支持 /v1/messages。
	StepFunMessagesUnsupportedModels = []string{
		"step-1o-turbo-vision",
	}
	// StepFunChatOnlyModels 仅支持 /v1/chat/completions。
	StepFunChatOnlyModels = []string{
		"step-1o-turbo-vision",
	}
)

// 音频系模型（用于模型↔端点标注；音乐/音频生成端点类型落地见后续阶段）。
var (
	StepFunSpeechModels = []string{
		"step-tts-mini",
		"step-tts-vivid",
		"step-tts-2",
		"stepaudio-2.5-tts",
		"stepaudio-3-tts",
	}
	StepFunTranscriptionModels = []string{
		"step-asr",
		"step-asr-1.1",
		"stepaudio-2-asr-pro",
		"stepaudio-2.5-asr",
		"stepaudio-3-asr-max",
	}
)

// StepFunListedModels 上游 GET /v1/models 全量快照（2026-09-19），用于渠道预填。
var StepFunListedModels = []string{
	"step-3.7-flash",
	"step-3.5-flash",
	"step-3.5-flash-2603",
	"step-router-v1",
	"step-gui",
	"step-1o-turbo-vision",
	"step-1o-audio",
	"step-2x-large",
	"step-image-edit-2",
	"step-audio-2",
	"step-audio-2-mini",
	"step-audio-2-think",
	"step-audio-r1.1",
	"step-audio-r1.5",
	"stepaudio-2.5-chat",
	"stepaudio-2.5-tts",
	"stepaudio-2.5-asr",
	"stepaudio-2.5-asr-stream",
	"stepaudio-2.5-realtime",
	"stepaudio-2-asr-pro",
	"stepaudio-3-asr-max",
	"stepaudio-3-chat-preview",
	"stepaudio-3-gen-preview",
	"stepaudio-3-music-preview",
	"stepaudio-3-realtime-preview",
	"stepaudio-3-tts",
	"step-tts-mini",
	"step-tts-vivid",
	"step-tts-2",
	"step-asr",
	"step-asr-1.1",
	"step-asr-1.1-stream",
	"step-overture-preview",
	"dr-search-api",
	"search-image",
}

// StepFunPseudoModels 非上游实体模型，仅用于无 model 字段的原生端点（Files、音色列表）路由。
var StepFunPseudoModels = []string{
	StepFunNativeFallbackModel,
}

// StepFunModelSupportsResponses 判断模型是否支持 /v1/responses（未收录模型视为支持）。
func StepFunModelSupportsResponses(modelName string) bool {
	return !modelInList(StepFunResponsesUnsupportedModels, modelName)
}

// StepFunModelSupportsMessages 判断模型是否支持 /v1/messages（未收录模型视为支持）。
func StepFunModelSupportsMessages(modelName string) bool {
	return !modelInList(StepFunMessagesUnsupportedModels, modelName)
}

func modelInList(list []string, modelName string) bool {
	for _, item := range list {
		if item == modelName {
			return true
		}
	}
	return false
}
