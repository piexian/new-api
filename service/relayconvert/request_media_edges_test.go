package relayconvert_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRemoteImagesPreserveDataAndDownloadFailures(t *testing.T) {
	previousMaxDownload := constant.MaxFileDownloadMB
	constant.MaxFileDownloadMB = 1
	t.Cleanup(func() { constant.MaxFileDownloadMB = previousMaxDownload })
	service.InitHttpClient()
	t.Cleanup(func() {
		service.GetHttpClient().CloseIdleConnections()
		service.GetSSRFProtectedHTTPClient().CloseIdleConnections()
	})
	data, err := base64.StdEncoding.DecodeString(imageData)
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/image" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(data)
	}))
	defer server.Close()
	parsedURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	fetch := system_setting.GetFetchSetting()
	previous := *fetch
	fetch.AllowPrivateIp = true
	fetch.AllowedPorts = append(append([]string(nil), fetch.AllowedPorts...), parsedURL.Port())
	t.Cleanup(func() { *fetch = previous })
	imageURL := server.URL + "/image"
	chat := &dto.GeneralOpenAIRequest{Model: "test", Messages: []dto.Message{{Role: "user", Content: []dto.MediaContent{{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: imageURL}}}}}}
	claude := convertRequest(t, chat, types.RelayFormatOpenAI, types.RelayFormatClaude)
	wire, err := common.Marshal(claude)
	require.NoError(t, err)
	require.Equal(t, imageData, gjson.GetBytes(wire, "messages.0.content.0.source.data").String())
	gemini := &dto.GeminiChatRequest{Contents: []dto.GeminiChatContent{{Role: "user", Parts: []dto.GeminiPart{{FileData: &dto.GeminiFileData{FileUri: imageURL}}}}}}
	wire, err = common.Marshal(convertRequest(t, gemini, types.RelayFormatGemini, types.RelayFormatOpenAI))
	require.NoError(t, err)
	require.Equal(t, "data:image/png;base64,"+imageData, gjson.GetBytes(wire, "messages.0.content.0.image_url.url").String())
	chat.Messages[0].SetMediaContent([]dto.MediaContent{{Type: dto.ContentTypeImageURL, ImageUrl: &dto.MessageImageUrl{Url: server.URL + "/missing"}}})
	result, err := relayconvert.ConvertRequest(nil, nil, types.RelayFormatClaude, chat)
	require.ErrorContains(t, err, "404")
	require.Nil(t, result)
}

func TestParallelToolResultsPrecedeAttachments(t *testing.T) {
	for _, source := range []types.RelayFormat{types.RelayFormatClaude, types.RelayFormatGemini} {
		t.Run(string(source), func(t *testing.T) {
			var req any
			var body string
			if source == types.RelayFormatClaude {
				req = &dto.ClaudeRequest{}
				body = `{"model":"test","messages":[{"role":"assistant","content":[{"type":"tool_use","id":"a","name":"first","input":{}},{"type":"tool_use","id":"b","name":"second","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"a","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"IMAGE"}}]}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"b","content":"second result"}]}]}`
			} else {
				req = &dto.GeminiChatRequest{}
				body = `{"contents":[{"role":"model","parts":[{"functionCall":{"id":"a","name":"first","args":{}}},{"functionCall":{"id":"b","name":"second","args":{}}}]},{"role":"user","parts":[{"functionResponse":{"id":"a","name":"first","response":{},"parts":[{"inlineData":{"mimeType":"image/png","data":"IMAGE"}}]}}]},{"role":"user","parts":[{"functionResponse":{"id":"b","name":"second","response":{"result":"second result"}}}]}]}`
			}
			require.NoError(t, common.Unmarshal([]byte(strings.ReplaceAll(body, "IMAGE", imageData)), req))
			converted := convertRequest(t, req, source, types.RelayFormatOpenAI).(*dto.GeneralOpenAIRequest)
			var roles []string
			for _, message := range converted.Messages {
				roles = append(roles, message.Role)
			}
			require.Equal(t, []string{"assistant", "tool", "tool", "user"}, roles)
			require.Equal(t, "a", converted.Messages[1].ToolCallId)
			require.Equal(t, "b", converted.Messages[2].ToolCallId)
		})
	}
}

func TestFileBlocksAndQuotedMessagesPreserveContent(t *testing.T) {
	pdf := "data:application/pdf;base64,JVBERi0xLjQKJUVPRgo="
	for _, target := range []types.RelayFormat{types.RelayFormatClaude, types.RelayFormatGemini, types.RelayFormatOpenAIResponses, types.RelayFormatGeminiInteractions} {
		t.Run(string(target), func(t *testing.T) {
			req := &dto.GeneralOpenAIRequest{Model: "test", Messages: []dto.Message{{Role: "user", Content: []dto.MediaContent{{Type: dto.ContentTypeFile, File: &dto.MessageFile{FileName: "report.pdf", FileData: pdf}}}}}}
			wire, err := common.Marshal(convertRequest(t, req, types.RelayFormatOpenAI, target))
			require.NoError(t, err)
			require.Contains(t, string(wire), "JVBERi0xLjQKJUVPRgo=")
			if target == types.RelayFormatClaude {
				require.Equal(t, "document", gjson.GetBytes(wire, "messages.0.content.0.type").String())
			}
			if target == types.RelayFormatOpenAIResponses {
				require.Equal(t, pdf, gjson.GetBytes(wire, "input.0.content.0.file_data").String())
				require.False(t, gjson.GetBytes(wire, "input.0.content.0.file").Exists())
			}
		})
	}
	req := &dto.GeneralOpenAIRequest{Model: "test", Stop: []string{"A", "B"}, Messages: []dto.Message{{Role: "user", Content: `"first`}, {Role: "user", Content: `last"`}}}
	converted := convertRequest(t, req, types.RelayFormatOpenAI, types.RelayFormatClaude).(*dto.ClaudeRequest)
	require.Equal(t, `"first last"`, converted.Messages[0].GetStringContent())
	require.Equal(t, []string{"A", "B"}, converted.StopSequences)
}
