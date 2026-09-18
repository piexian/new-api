package stepfun

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// NativeEndpoint 描述一个 StepFun 原生（非 OpenAI 标准）端点的透传契约。
//
// 入站路径与上游路径 1:1 对应，因此 new-api 只负责：① 用 model 完成渠道选择；
// ② 把仅用于路由的 model 字段从转发体/查询串中剥离（上游 schema 没有该字段时）；
// ③ 原样回传响应；④ 对提交类端点计费，查询/列表类不重复计费。
//
// 端点可用性来自 2026-09-19 实测矩阵：Step Plan 通道只提供 speech / asr-sse /
// voices / voices(列表)，音乐、音频生成、ASR 文件异步、system_voices 仅开放平台提供。
type NativeEndpoint struct {
	Path           string
	Method         string
	ModelKeys      []string // 请求体 JSON 路径（支持 gjson 点号路径），按序取第一个非空值
	ModelQuery     bool     // 允许以 ?model= 兜底解析路由模型
	KeepModelQuery bool     // 转发时保留 model 查询参数（上游真实字段，如 system_voices）
	StripBodyModel bool     // 转发时剥离顶层 model 字段（上游 schema 无该字段）
	Billable       bool     // 提交类端点计费；查询/列表类不计费
	StepPlanOnly   bool     // 仅 Step Plan 通道提供（当前为空，保留语义位）
	OpenPlatform   bool     // 仅开放平台提供（Step Plan 通道上游 404）
}

var stepFunNativeEndpoints = []NativeEndpoint{
	{
		Path:         "/v1/audio/generate",
		Method:       http.MethodPost,
		ModelKeys:    []string{"model"},
		Billable:     true,
		OpenPlatform: true,
	},
	{
		Path:           "/v1/audio/music/submit",
		Method:         http.MethodPost,
		ModelKeys:      []string{"model", "model_id"},
		StripBodyModel: true,
		Billable:       true,
		OpenPlatform:   true,
	},
	{
		Path:           "/v1/audio/music/query",
		Method:         http.MethodPost,
		ModelKeys:      []string{"model"},
		ModelQuery:     true,
		StripBodyModel: true,
		OpenPlatform:   true,
	},
	{
		Path:           "/v1/audio/asr/sse",
		Method:         http.MethodPost,
		ModelKeys:      []string{"model", "audio.input.transcription.model"},
		StripBodyModel: true,
		Billable:       true,
	},
	{
		Path:           "/v1/audio/asr/file/submit",
		Method:         http.MethodPost,
		ModelKeys:      []string{"model", "request.model_name"},
		StripBodyModel: true,
		Billable:       true,
		OpenPlatform:   true,
	},
	{
		Path:           "/v1/audio/asr/file/query",
		Method:         http.MethodPost,
		ModelKeys:      []string{"model"},
		ModelQuery:     true,
		StripBodyModel: true,
		OpenPlatform:   true,
	},
	{
		Path:      "/v1/audio/voices",
		Method:    http.MethodPost,
		ModelKeys: []string{"model"},
		Billable:  true,
	},
	{
		Path:       "/v1/audio/voices",
		Method:     http.MethodGet,
		ModelKeys:  []string{"model"},
		ModelQuery: true,
	},
	{
		Path:         "/v1/audio/voices/preview",
		Method:       http.MethodPost,
		ModelKeys:    []string{"model"},
		Billable:     true,
		OpenPlatform: true,
	},
	{
		Path:           "/v1/audio/system_voices",
		Method:         http.MethodGet,
		ModelKeys:      []string{"model"},
		ModelQuery:     true,
		KeepModelQuery: true,
		OpenPlatform:   true,
	},
	// 文件端点（音色复刻前置依赖）：上游为 OpenAI Files 形状，无 model 字段，
	// 由渠道配置的 fallback 模型完成路由。
	{Path: "/v1/files", Method: http.MethodPost, Billable: true, OpenPlatform: true},
	{Path: "/v1/files", Method: http.MethodGet, OpenPlatform: true},
	{Path: "/v1/files/:id", Method: http.MethodGet, OpenPlatform: true},
	{Path: "/v1/files/:id", Method: http.MethodDelete, OpenPlatform: true},
	{Path: "/v1/files/:id/content", Method: http.MethodGet, OpenPlatform: true},
}

// WssEndpoint 描述 StepFun WebSocket 原生端点：网关只做原始双向转发，
// 不改写任何事件（三套 WS 协议各不相同，逐条解析没有收益）。
type WssEndpoint struct {
	Path               string
	RequiresModelQuery bool // 上游要求以 ?model= 指定模型
	OpenPlatform       bool // 仅开放平台提供（Step Plan 通道上游 404）
}

var stepFunWssEndpoints = []WssEndpoint{
	{Path: "/v1/realtime/audio", RequiresModelQuery: true},                       // 流式语音合成
	{Path: "/v1/audio/asr/stream", RequiresModelQuery: true, OpenPlatform: true}, // 双向流式识别
	{Path: "/v1/realtime", RequiresModelQuery: true},                             // 双向实时语音
}

// LookupWssEndpoint 按路径匹配 WebSocket 原生端点。
func LookupWssEndpoint(path string) (WssEndpoint, bool) {
	normalizedPath := normalizeNativePath(path)
	for _, endpoint := range stepFunWssEndpoints {
		if endpoint.Path == normalizedPath {
			return endpoint, true
		}
	}
	return WssEndpoint{}, false
}

// IsWssPath 判断路径是否为 StepFun WebSocket 原生端点。
func IsWssPath(path string) bool {
	_, ok := LookupWssEndpoint(path)
	return ok
}

// AvailableOnBase 判断 WebSocket 端点是否在当前 base 上可用。
func (e WssEndpoint) AvailableOnBase(rawBase string) bool {
	if !e.OpenPlatform {
		return true
	}
	return !IsStepPlanBase(rawBase)
}

// WssEndpointTableSize 暴露 WebSocket 端点表规模，供测试与诊断使用。
func WssEndpointTableSize() int {
	return len(stepFunWssEndpoints)
}

// IsRealtimeModel 判断模型是否属于 StepFun 双向实时语音系列（用于 /v1/realtime 路由分派）。
func IsRealtimeModel(modelName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(modelName))
	return strings.HasPrefix(normalized, "stepaudio-") && strings.Contains(normalized, "realtime")
}

// WebsocketBaseURL 将 http(s) base 转为 ws(s) base。
func WebsocketBaseURL(baseURL string) string {
	switch {
	case strings.HasPrefix(baseURL, "https://"):
		return "wss://" + strings.TrimPrefix(baseURL, "https://")
	case strings.HasPrefix(baseURL, "http://"):
		return "ws://" + strings.TrimPrefix(baseURL, "http://")
	default:
		return baseURL
	}
}

// LookupNativeEndpoint 按入站方法与路径匹配原生端点定义。
func LookupNativeEndpoint(path, method string) (NativeEndpoint, bool) {
	normalizedPath := normalizeNativePath(path)
	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	for _, endpoint := range stepFunNativeEndpoints {
		if endpoint.Method == normalizedMethod && nativePathMatches(endpoint.Path, normalizedPath) {
			return endpoint, true
		}
	}
	return NativeEndpoint{}, false
}

// IsNativePath 判断路径是否属于 StepFun 原生端点（不校验方法）。
func IsNativePath(path string) bool {
	normalizedPath := normalizeNativePath(path)
	for _, endpoint := range stepFunNativeEndpoints {
		if nativePathMatches(endpoint.Path, normalizedPath) {
			return true
		}
	}
	return false
}

// NativeRouteModel 依次从请求体（JSON 点号路径）与查询参数解析用于渠道路由的模型。
func NativeRouteModel(body []byte, query url.Values, endpoint NativeEndpoint) string {
	if len(body) > 0 && gjson.ValidBytes(body) {
		for _, key := range endpoint.ModelKeys {
			if value := strings.TrimSpace(gjson.GetBytes(body, key).String()); value != "" {
				return value
			}
		}
	}
	if endpoint.ModelQuery && query != nil {
		if value := strings.TrimSpace(query.Get("model")); value != "" {
			return value
		}
	}
	return ""
}

// PrepareNativeRequestBody 按端点定义裁剪转发体：剥离仅用于渠道路由的顶层 model。
func PrepareNativeRequestBody(body []byte, endpoint NativeEndpoint) ([]byte, error) {
	if !endpoint.StripBodyModel || len(body) == 0 || !gjson.ValidBytes(body) {
		return body, nil
	}
	if !gjson.GetBytes(body, "model").Exists() {
		return body, nil
	}
	return sjson.DeleteBytes(body, "model")
}

// NativeUpstreamQuery 归一转发查询串：路由专用的 model 参数在上游无对应字段时剥离。
func NativeUpstreamQuery(rawQuery string, endpoint NativeEndpoint) string {
	if rawQuery == "" || endpoint.KeepModelQuery {
		return rawQuery
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}
	if values.Has("model") {
		values.Del("model")
	}
	return values.Encode()
}

// NativeUpstreamPath 返回转发到上游的路径（含归一后的查询串）。
func NativeUpstreamPath(path, rawQuery string, endpoint NativeEndpoint) string {
	upstreamPath := normalizeNativePath(path)
	if query := NativeUpstreamQuery(rawQuery, endpoint); query != "" {
		return upstreamPath + "?" + query
	}
	return upstreamPath
}

// AvailableOnBase 判断端点是否在当前 base（Step Plan / 开放平台）上可用。
func (e NativeEndpoint) AvailableOnBase(rawBase string) bool {
	if !e.OpenPlatform {
		return true
	}
	return !IsStepPlanBase(rawBase)
}

// IsStepPlanBase 判断渠道 base 是否为 Step Plan 通道。
func IsStepPlanBase(rawBase string) bool {
	base, ok := resolveBase(rawBase)
	return ok && base.plan
}

// NativeEndpointTableSize 暴露端点表规模，供测试与诊断使用。
func NativeEndpointTableSize() int {
	return len(stepFunNativeEndpoints)
}

func normalizeNativePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	if len(trimmed) > 1 {
		trimmed = strings.TrimRight(trimmed, "/")
	}
	return trimmed
}

// nativePathMatches 支持 :id 形式的单段参数匹配（仅文件端点使用）。
func nativePathMatches(pattern, path string) bool {
	if pattern == path {
		return true
	}
	if !strings.Contains(pattern, ":") {
		return false
	}
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(patternParts) != len(pathParts) {
		return false
	}
	for i := range patternParts {
		if strings.HasPrefix(patternParts[i], ":") {
			if pathParts[i] == "" {
				return false
			}
			continue
		}
		if patternParts[i] != pathParts[i] {
			return false
		}
	}
	return true
}

// SplitPathQuery 拆分「路径?查询串」，供原生端点识别与转发使用。
func SplitPathQuery(pathWithQuery string) (string, string) {
	trimmed := strings.TrimSpace(pathWithQuery)
	if index := strings.Index(trimmed, "?"); index >= 0 {
		return trimmed[:index], trimmed[index+1:]
	}
	return trimmed, ""
}

// lookupNativeByPath 按路径匹配端点（方法无关，取首个匹配项）。
func lookupNativeByPath(path string) (NativeEndpoint, bool) {
	normalizedPath := normalizeNativePath(path)
	for _, endpoint := range stepFunNativeEndpoints {
		if nativePathMatches(endpoint.Path, normalizedPath) {
			return endpoint, true
		}
	}
	return NativeEndpoint{}, false
}

// NativeForwardPath 返回原生端点转发到上游的路径（入站路径 1:1，查询串按端点定义归一）。
func NativeForwardPath(pathWithQuery string) string {
	path, rawQuery := SplitPathQuery(pathWithQuery)
	endpoint, ok := lookupNativeByPath(path)
	if !ok {
		return pathWithQuery
	}
	return NativeUpstreamPath(path, rawQuery, endpoint)
}
