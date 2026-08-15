package mistralconsole

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	clientStream bool
}

// boraContinueAssistantInstruction 是尾随 assistant 消息（prefill）被拒绝时追加的
// 用户续写输入，避免把上游晦涩的 422/code 3000 直接透传给客户端。
const boraContinueAssistantInstruction = "Please continue."

// boraProtectedFunctionSuffix 是 function 工具与 bora 内置工具类型重名时的重命名后缀。
const boraProtectedFunctionSuffix = "_fn"

// isBoraProtectedFunctionName 判断函数名是否撞上了 bora 的内置工具类型名。
// 上游只允许这些名字出现在内置工具上，作为 function 工具名会返回 422
// （生产证据：value_error: The following tool has a protected function name: web_search）。
func isBoraProtectedFunctionName(name string) bool {
	switch name {
	case "web_search", "code_interpreter", "image_generation", "web_search_premium":
		return true
	}
	return false
}

// boraFunctionNameForUpstream 返回发送给上游的函数名：与 bora 内置工具类型名冲突时
// 追加 _fn 后缀重命名。重命名是确定性的，响应侧可按 boraFunctionNameForClient 逆映射。
func boraFunctionNameForUpstream(name string) string {
	if isBoraProtectedFunctionName(name) {
		return name + boraProtectedFunctionSuffix
	}
	return name
}

// boraFunctionNameForClient 返回给客户端看的函数名：仅当名字形如 "<内置工具类型名>_fn"
// 时才裁掉后缀。客户端真把工具命名为 "web_search_fn" 这类名字时会被误裁回
// "web_search"，属可接受的极小概率误判（客户端几乎不会用内置工具类型名命名函数）。
func boraFunctionNameForClient(name string) string {
	base, found := strings.CutSuffix(name, boraProtectedFunctionSuffix)
	if found && isBoraProtectedFunctionName(base) {
		return base
	}
	return name
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.clientStream = info != nil && info.IsStream
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("relay info is nil")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(info.ChannelBaseUrl), "/")
	if baseURL == "" {
		return "", errors.New("upstream base URL is empty")
	}
	return baseURL + conversationsURL, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	if info == nil {
		return errors.New("relay info is nil")
	}
	cookie, err := validateCookieHeaderValue(info.ApiKey)
	if err != nil {
		return err
	}
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Accept", "text/event-stream")
	req.Set("Content-Type", "application/json")
	req.Set("Cookie", cookie)
	req.Del("Authorization")
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(_ *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, invalidRequestError("request is nil")
	}
	if info == nil {
		return nil, invalidRequestError("relay info is nil")
	}
	if _, err := validateCookieHeaderValue(info.ApiKey); err != nil {
		return nil, invalidRequestError(err.Error())
	}
	if rawJSONHasValue(request.Functions) || rawJSONHasValue(request.FunctionCall) {
		return nil, invalidRequestError("upstream does not support legacy functions or function_call")
	}

	instructions, inputs, err := convertBoraInputs(request.Messages)
	if err != nil {
		return nil, invalidRequestError(err.Error())
	}
	if len(inputs) == 0 {
		return nil, invalidRequestError("upstream requires at least one message or function result")
	}

	tools, toolInstruction, err := convertBoraTools(request.Tools, request.ToolChoice)
	if err != nil {
		return nil, invalidRequestError(err.Error())
	}
	if strings.TrimSpace(request.Instruction) != "" {
		instructions = appendInstruction(instructions, "[instruction]\n"+request.Instruction)
	}
	if toolInstruction != "" {
		instructions = appendInstruction(instructions, toolInstruction)
	}

	maxTokens := boraMaxTokens(request)
	return &boraConversationRequest{
		Model:        info.UpstreamModelName,
		Instructions: instructions,
		CompletionArgs: boraCompletionArgs{
			Temperature: normalizeBoraTemperature(request.Temperature),
			MaxTokens:   &maxTokens,
			TopP:        normalizeBoraTopP(request.TopP),
			// Bora only accepts none/high. high is its maximum reasoning level,
			// so every downstream reasoning setting is normalized to high.
			ReasoningEffort: boraMaxReasoningEffort,
		},
		Tools:  tools,
		Stream: true,
		Inputs: inputs,
	}, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info == nil {
		return nil, badResponseError(errors.New("relay info is nil"))
	}
	// The upstream always responds with SSE. Restore the original client mode
	// after the relay's Content-Type detection changed info.IsStream.
	info.IsStream = a.clientStream
	if a.clientStream {
		return handleBoraStreamResponse(c, resp, info)
	}
	return handleBoraResponse(c, resp, info)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("upstream only supports OpenAI chat completions")
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("upstream only supports OpenAI chat completions")
}

func (a *Adaptor) ConvertEmbeddingRequest(*gin.Context, *relaycommon.RelayInfo, dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("upstream does not support embeddings")
}

func (a *Adaptor) ConvertAudioRequest(*gin.Context, *relaycommon.RelayInfo, dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("upstream does not support audio")
}

func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, errors.New("upstream does not support images")
}

func (a *Adaptor) ConvertRerankRequest(*gin.Context, int, dto.RerankRequest) (any, error) {
	return nil, errors.New("upstream does not support reranking")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(*gin.Context, *relaycommon.RelayInfo, dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("upstream does not support the Responses API")
}

func validateCookieHeaderValue(value string) (string, error) {
	cookie := strings.TrimSpace(value)
	if cookie == "" {
		return "", errors.New("upstream Cookie is empty")
	}
	if strings.ContainsAny(cookie, "\r\n") {
		return "", errors.New("upstream Cookie must not contain CR or LF characters")
	}
	if strings.HasPrefix(strings.ToLower(cookie), "cookie:") {
		return "", errors.New("enter only the Cookie header value, without the Cookie: prefix")
	}
	return cookie, nil
}

func invalidRequestError(message string) error {
	return types.NewErrorWithStatusCode(
		errors.New(message),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

func rawJSONHasValue(value []byte) bool {
	trimmed := strings.TrimSpace(string(value))
	return trimmed != "" && trimmed != "null" && trimmed != "[]" && trimmed != "{}"
}

func boraMaxTokens(request *dto.GeneralOpenAIRequest) uint {
	value := defaultBoraMaxTokens
	if request.MaxCompletionTokens != nil {
		value = *request.MaxCompletionTokens
	} else if request.MaxTokens != nil {
		value = *request.MaxTokens
	}
	if value > defaultBoraMaxTokens {
		return defaultBoraMaxTokens
	}
	return value
}

// normalizeBoraTemperature 把 temperature 钳制到 bora 接受的 [0,1] 区间，nil 保持不传。
func normalizeBoraTemperature(value *float64) *float64 {
	if value == nil {
		return nil
	}
	normalized := *value
	if normalized < 0 {
		normalized = 0
	} else if normalized > 1 {
		normalized = 1
	}
	return &normalized
}

func normalizeBoraTopP(value *float64) *float64 {
	if value == nil {
		return nil
	}
	normalized := *value
	// bora 的 schema 声称 top_p=0 合法，但模型后端会对 0 返回 422；取最接近的可用正值。
	// 上界钳制到 1。
	if normalized <= 0 {
		normalized = 0.0001
	} else if normalized > 1 {
		normalized = 1
	}
	return &normalized
}

func convertBoraInputs(messages []dto.Message) (string, []boraInput, error) {
	instructionParts := make([]string, 0)
	inputs := make([]boraInput, 0, len(messages))
	falseValue := false

	for index := range messages {
		message := &messages[index]
		// 剥离 prefix/reasoning 字段——bora API 不支持，不传给上游

		text, err := textMessageContent(message.Content)
		if err != nil {
			return "", nil, fmt.Errorf("message %d: %w", index, err)
		}

		switch message.Role {
		case "system", "developer":
			if rawJSONHasValue(message.ToolCalls) || message.ToolCallId != "" {
				return "", nil, fmt.Errorf("message %d: %s messages cannot contain tool calls", index, message.Role)
			}
			if text != "" {
				instructionParts = append(instructionParts, labeledText(message.Role, message.Name, text))
			}
		case "user":
			if rawJSONHasValue(message.ToolCalls) || message.ToolCallId != "" {
				return "", nil, fmt.Errorf("message %d: user messages cannot contain tool calls", index)
			}
			if text != "" {
				content := namedConversationText("user", message.Name, text)
				inputs = append(inputs, boraInput{
					Object:  "entry",
					Type:    "message.input",
					Role:    "user",
					Content: &content,
					Prefix:  &falseValue,
				})
			}
		case "assistant":
			if message.ToolCallId != "" {
				return "", nil, fmt.Errorf("message %d: assistant messages cannot contain tool_call_id", index)
			}
			// 历史 reasoning 不回传：bora 输入不支持结构化 thinking，
			// 拼成文本会导致模型模仿 "[thinking]" 格式并漏进正文
			if text != "" {
				content := namedConversationText("assistant", message.Name, text)
				inputs = append(inputs, boraInput{
					Object:  "entry",
					Type:    "message.output",
					Role:    "assistant",
					Content: &content,
				})
			}
			if rawJSONHasValue(message.ToolCalls) {
				toolCallInputs, err := convertAssistantToolCalls(message.ToolCalls)
				if err != nil {
					return "", nil, fmt.Errorf("message %d: %w", index, err)
				}
				inputs = append(inputs, toolCallInputs...)
			}
		case "tool":
			if rawJSONHasValue(message.ToolCalls) {
				return "", nil, fmt.Errorf("message %d: tool result messages cannot contain tool_calls", index)
			}
			if strings.TrimSpace(message.ToolCallId) == "" {
				return "", nil, fmt.Errorf("message %d: tool result requires tool_call_id", index)
			}
			result := text
			inputs = append(inputs, boraInput{
				Object:     "entry",
				Type:       "function.result",
				ToolCallID: message.ToolCallId,
				Result:     &result,
			})
		case "function":
			return "", nil, fmt.Errorf("message %d: legacy function role is not supported; use tool messages", index)
		default:
			return "", nil, fmt.Errorf("upstream does not support message role %q", message.Role)
		}
	}
	// bora 拒绝最后一条输入是 message.output 的会话（上游 422/code 3000）。
	// 部分 OpenAI 客户端习惯以 assistant 消息结尾来请求续写（prefill），
	// 这里保留该输出并追加一条明确的用户续写输入，而不是透传上游的报错。
	if len(inputs) > 0 && inputs[len(inputs)-1].Type == "message.output" {
		content := boraContinueAssistantInstruction
		inputs = append(inputs, boraInput{
			Object:  "entry",
			Type:    "message.input",
			Role:    "user",
			Content: &content,
			Prefix:  &falseValue,
		})
	}
	return strings.Join(instructionParts, "\n\n"), inputs, nil
}

func convertAssistantToolCalls(raw []byte) ([]boraInput, error) {
	var toolCalls []dto.ToolCallRequest
	if err := common.Unmarshal(raw, &toolCalls); err != nil {
		return nil, fmt.Errorf("invalid tool_calls: %w", err)
	}
	if len(toolCalls) == 0 {
		return nil, errors.New("tool_calls must contain at least one function call")
	}
	inputs := make([]boraInput, 0, len(toolCalls))
	for index := range toolCalls {
		toolCall := &toolCalls[index]
		if toolCall.Type != "function" {
			return nil, fmt.Errorf("tool call %d has unsupported type %q", index, toolCall.Type)
		}
		if strings.TrimSpace(toolCall.ID) == "" || strings.TrimSpace(toolCall.Function.Name) == "" {
			return nil, fmt.Errorf("tool call %d requires id and function.name", index)
		}
		arguments := toolCall.Function.Arguments
		inputs = append(inputs, boraInput{
			Object:     "entry",
			Type:       "function.call",
			Name:       toolCall.Function.Name,
			ToolCallID: toolCall.ID,
			Arguments:  &arguments,
		})
	}
	return inputs, nil
}

func convertBoraTools(openAITools []dto.ToolCallRequest, toolChoice any) ([]boraTool, string, error) {
	tools := make([]boraTool, 0, len(openAITools))
	for index := range openAITools {
		tool := &openAITools[index]
		switch tool.Type {
		case "function":
			if strings.TrimSpace(tool.Function.Name) == "" {
				return nil, "", fmt.Errorf("tool %d requires function.name", index)
			}
			// bora 内置工具类型名不允许作为 function 工具名（上游 422），
			// 冲突时追加 _fn 后缀重命名，响应侧再逆映射回客户端原始名称。
			name := boraFunctionNameForUpstream(tool.Function.Name)
			// bora 要求 parameters 必填，无参数工具给空 object schema。
			// 客户端传 "parameters": {} 时 Go 端是非 nil 的空 map，omitempty 序列化
			// 会把它整个丢掉导致上游 422，因此空 map 也必须替换为默认 schema
			parameters := tool.Function.Parameters
			if m, ok := parameters.(map[string]any); parameters == nil || (ok && len(m) == 0) {
				parameters = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			tools = append(tools, boraTool{
				Type: "function",
				Function: &boraFunction{
					Name:        name,
					Description: tool.Function.Description,
					Parameters:  parameters,
					Strict:      tool.Function.Strict,
				},
			})
		case "code_interpreter", "image_generation", "web_search_premium":
			tools = append(tools, boraTool{Type: tool.Type})
		case "web_search", "web_search_preview":
			tools = append(tools, boraTool{Type: "web_search_premium"})
		default:
			return nil, "", fmt.Errorf("tool %d has unsupported type %q", index, tool.Type)
		}
	}

	return applyBoraToolChoice(tools, toolChoice)
}

func applyBoraToolChoice(tools []boraTool, choice any) ([]boraTool, string, error) {
	if choice == nil {
		return tools, "", nil
	}
	if value, ok := choice.(string); ok {
		switch value {
		case "auto":
			return tools, "", nil
		case "none":
			return nil, "", nil
		case "required":
			if len(tools) == 0 {
				return nil, "", errors.New("tool_choice required needs at least one tool")
			}
			return tools, "You must call at least one provided tool before answering.", nil
		default:
			return nil, "", fmt.Errorf("unsupported tool_choice %q", value)
		}
	}

	var selected struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	data, err := common.Marshal(choice)
	if err != nil {
		return nil, "", fmt.Errorf("invalid tool_choice: %w", err)
	}
	if err := common.Unmarshal(data, &selected); err != nil {
		return nil, "", fmt.Errorf("invalid tool_choice: %w", err)
	}
	if selected.Type != "function" || strings.TrimSpace(selected.Function.Name) == "" {
		return nil, "", errors.New("upstream only supports named function object tool_choice")
	}
	// 与 convertBoraTools 保持一致的重命名规则，否则重命名后的工具永远匹配不上
	selected.Function.Name = boraFunctionNameForUpstream(selected.Function.Name)
	for _, tool := range tools {
		if tool.Type == "function" && tool.Function != nil && tool.Function.Name == selected.Function.Name {
			return []boraTool{tool}, "You must call the function " + selected.Function.Name + " before answering.", nil
		}
	}
	return nil, "", fmt.Errorf("tool_choice references unknown function %q", selected.Function.Name)
}

func appendInstruction(current string, extra string) string {
	if current == "" {
		return extra
	}
	return current + "\n\n" + extra
}

func labeledText(role string, name *string, text string) string {
	label := role
	if name != nil && strings.TrimSpace(*name) != "" {
		label += ":" + strings.TrimSpace(*name)
	}
	return "[" + label + "]\n" + text
}

func namedConversationText(role string, name *string, text string) string {
	if name == nil || strings.TrimSpace(*name) == "" {
		return text
	}
	return labeledText(role, name, text)
}

func textMessageContent(content any) (string, error) {
	switch value := content.(type) {
	case nil:
		return "", nil
	case string:
		return value, nil
	case []any:
		var builder strings.Builder
		for _, item := range value {
			text, err := textContentPart(item)
			if err != nil {
				return "", err
			}
			builder.WriteString(text)
		}
		return builder.String(), nil
	case []dto.MediaContent:
		var builder strings.Builder
		for _, item := range value {
			text, err := textContentPart(item)
			if err != nil {
				return "", err
			}
			builder.WriteString(text)
		}
		return builder.String(), nil
	default:
		return "", fmt.Errorf("unsupported message content type %T", content)
	}
}

func textContentPart(part any) (string, error) {
	switch value := part.(type) {
	case dto.MediaContent:
		if value.Type != dto.ContentTypeText {
			return "", fmt.Errorf("upstream does not support content type %q", value.Type)
		}
		return value.Text, nil
	case map[string]any:
		contentType, ok := value["type"].(string)
		if !ok || contentType != dto.ContentTypeText {
			return "", fmt.Errorf("upstream does not support content type %q", contentType)
		}
		text, ok := value["text"].(string)
		if !ok {
			return "", errors.New("text content must be a string")
		}
		return text, nil
	default:
		return "", fmt.Errorf("unsupported content part type %T", part)
	}
}
