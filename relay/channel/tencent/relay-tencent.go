package tencent

import (
	"bufio"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/responsescompat"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// https://cloud.tencent.com/document/product/1729/97732

func requestOpenAI2Tencent(a *Adaptor, request dto.GeneralOpenAIRequest) *TencentChatRequest {
	messages := make([]*TencentMessage, 0, len(request.Messages))
	for i := 0; i < len(request.Messages); i++ {
		message := request.Messages[i]
		messages = append(messages, &TencentMessage{
			Content: message.StringContent(),
			Role:    message.Role,
		})
	}
	var req = TencentChatRequest{
		Stream:   request.Stream,
		Messages: messages,
		Model:    &request.Model,
	}
	if request.TopP != nil {
		req.TopP = request.TopP
	}
	req.Temperature = request.Temperature
	return &req
}

func responseTencent2OpenAI(response *TencentChatResponse) *dto.OpenAITextResponse {
	fullTextResponse := dto.OpenAITextResponse{
		Id:      response.Id,
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Usage: dto.Usage{
			PromptTokens:     response.Usage.PromptTokens,
			CompletionTokens: response.Usage.CompletionTokens,
			TotalTokens:      response.Usage.TotalTokens,
		},
	}
	if len(response.Choices) > 0 {
		choice := dto.OpenAITextResponseChoice{
			Index: 0,
			Message: dto.Message{
				Role:    "assistant",
				Content: response.Choices[0].Messages.Content,
			},
			FinishReason: response.Choices[0].FinishReason,
		}
		fullTextResponse.Choices = append(fullTextResponse.Choices, choice)
	}
	return &fullTextResponse
}

func tencentResponseBody2OpenAI(resp *http.Response) (*dto.OpenAITextResponse, *dto.Usage, *types.NewAPIError) {
	var tencentSb TencentChatResponseSB
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)
	if err = common.Unmarshal(responseBody, &tencentSb); err != nil {
		return nil, nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if tencentSb.Response.Error.Code != 0 {
		return nil, nil, types.WithOpenAIError(types.OpenAIError{
			Message: tencentSb.Response.Error.Message,
			Code:    tencentSb.Response.Error.Code,
		}, resp.StatusCode)
	}
	fullTextResponse := responseTencent2OpenAI(&tencentSb.Response)
	return fullTextResponse, &fullTextResponse.Usage, nil
}

func streamResponseTencent2OpenAI(TencentResponse *TencentChatResponse) *dto.ChatCompletionsStreamResponse {
	response := dto.ChatCompletionsStreamResponse{
		Object:  "chat.completion.chunk",
		Created: common.GetTimestamp(),
		Model:   "tencent-hunyuan",
	}
	if len(TencentResponse.Choices) > 0 {
		var choice dto.ChatCompletionsStreamResponseChoice
		choice.Delta.SetContentString(TencentResponse.Choices[0].Delta.Content)
		if TencentResponse.Choices[0].FinishReason == "stop" {
			choice.FinishReason = &constant.FinishReasonStop
		}
		response.Choices = append(response.Choices, choice)
	}
	return &response
}

func tencentStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	var responseText string
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	helper.SetEventStreamHeaders(c)

	for scanner.Scan() {
		data := scanner.Text()
		if len(data) < 5 || !strings.HasPrefix(data, "data:") {
			continue
		}
		data = strings.TrimPrefix(data, "data:")

		var tencentResponse TencentChatResponse
		err := common.Unmarshal([]byte(data), &tencentResponse)
		if err != nil {
			common.SysLog("error unmarshalling stream response: " + err.Error())
			continue
		}

		response := streamResponseTencent2OpenAI(&tencentResponse)
		if len(response.Choices) != 0 {
			responseText += response.Choices[0].Delta.GetContentString()
		}

		err = helper.ObjectData(c, response)
		if err != nil {
			common.SysLog(err.Error())
		}
	}

	if err := scanner.Err(); err != nil {
		common.SysLog("error reading stream: " + err.Error())
	}

	helper.Done(c)

	service.CloseResponseBodyGracefully(resp)

	return service.ResponseText2Usage(c, responseText, info.UpstreamModelName, info.GetEstimatePromptTokens()), nil
}

func tencentResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)
	helper.SetEventStreamHeaders(c)
	emitter := responsescompat.NewStreamEmitter(c, info)

	for scanner.Scan() {
		data := scanner.Text()
		if len(data) < 5 || !strings.HasPrefix(data, "data:") {
			continue
		}
		data = strings.TrimSpace(strings.TrimPrefix(data, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var tencentResponse TencentChatResponse
		err := common.Unmarshal(common.StringToByteSlice(data), &tencentResponse)
		if err != nil {
			common.SysLog("error unmarshalling tencent responses stream response: " + err.Error())
			continue
		}
		if tencentResponse.Id != "" {
			emitter.SetResponseID(tencentResponse.Id)
		}
		if tencentResponse.Created != 0 {
			emitter.SetCreatedAt(tencentResponse.Created)
		}
		if tencentResponse.Usage.TotalTokens != 0 || tencentResponse.Usage.PromptTokens != 0 || tencentResponse.Usage.CompletionTokens != 0 {
			usage := dto.Usage{
				PromptTokens:     tencentResponse.Usage.PromptTokens,
				CompletionTokens: tencentResponse.Usage.CompletionTokens,
				TotalTokens:      tencentResponse.Usage.TotalTokens,
			}
			emitter.SetUsage(&usage)
		}
		if len(tencentResponse.Choices) == 0 {
			continue
		}
		choice := tencentResponse.Choices[0]
		if !emitter.SendTextDelta(choice.Delta.Content) {
			service.CloseResponseBodyGracefully(resp)
			return nil, emitter.Err()
		}
		if choice.FinishReason != "" {
			if !emitter.Complete() {
				service.CloseResponseBodyGracefully(resp)
				return nil, emitter.Err()
			}
		}
	}

	if err := scanner.Err(); err != nil {
		common.SysLog("error reading tencent responses stream: " + err.Error())
	}
	service.CloseResponseBodyGracefully(resp)
	if emitter.Err() != nil {
		return nil, emitter.Err()
	}
	if !emitter.Complete() {
		return nil, emitter.Err()
	}
	return emitter.Usage(), nil
}

func tencentHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	fullTextResponse, usage, apiErr := tencentResponseBody2OpenAI(resp)
	if apiErr != nil {
		return nil, apiErr
	}
	jsonResponse, err := common.Marshal(fullTextResponse)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	service.IOCopyBytesGracefully(c, resp, jsonResponse)
	return usage, nil
}

func tencentResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	fullTextResponse, _, apiErr := tencentResponseBody2OpenAI(resp)
	if apiErr != nil {
		return nil, apiErr
	}
	responsesResponse, usage := responsescompat.ChatCompletionToResponse(c, info, fullTextResponse)
	jsonResponse, err := common.Marshal(responsesResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, jsonResponse)
	return usage, nil
}

func parseTencentConfig(config string) (appId int64, secretId string, secretKey string, err error) {
	parts := strings.Split(config, "|")
	if len(parts) != 3 {
		err = errors.New("invalid tencent config")
		return
	}
	appId, err = strconv.ParseInt(parts[0], 10, 64)
	secretId = parts[1]
	secretKey = parts[2]
	return
}

func sha256hex(s string) string {
	b := sha256.Sum256([]byte(s))
	return hex.EncodeToString(b[:])
}

func hmacSha256(s, key string) string {
	hashed := hmac.New(sha256.New, []byte(key))
	hashed.Write([]byte(s))
	return string(hashed.Sum(nil))
}

func getTencentSign(req TencentChatRequest, adaptor *Adaptor, secId, secKey string) string {
	// build canonical request string
	host := "hunyuan.tencentcloudapi.com"
	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-tc-action:%s\n",
		"application/json", host, strings.ToLower(adaptor.Action))
	signedHeaders := "content-type;host;x-tc-action"
	payload, _ := json.Marshal(req)
	hashedRequestPayload := sha256hex(string(payload))
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedRequestPayload)
	// build string to sign
	algorithm := "TC3-HMAC-SHA256"
	requestTimestamp := strconv.FormatInt(adaptor.Timestamp, 10)
	timestamp, _ := strconv.ParseInt(requestTimestamp, 10, 64)
	t := time.Unix(timestamp, 0).UTC()
	// must be the format 2006-01-02, ref to package time for more info
	date := t.Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, "hunyuan")
	hashedCanonicalRequest := sha256hex(canonicalRequest)
	string2sign := fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		requestTimestamp,
		credentialScope,
		hashedCanonicalRequest)

	// sign string
	secretDate := hmacSha256(date, "TC3"+secKey)
	secretService := hmacSha256("hunyuan", secretDate)
	secretKey := hmacSha256("tc3_request", secretService)
	signature := hex.EncodeToString([]byte(hmacSha256(string2sign, secretKey)))

	// build authorization
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		secId,
		credentialScope,
		signedHeaders,
		signature)
	return authorization
}
