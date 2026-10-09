package controller

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

// testUpstreamCaptureLimit 限制为提取上游模型名而保留的响应字节数，渠道测试的
// 响应很小，上限只是防御。
const testUpstreamCaptureLimit = 4 << 20

// testUpstreamCapture 透传并记录上游响应体，用于在渠道测试中展示上游实际返回的
// 模型名（适配层转换或模型映射前的值）。
type testUpstreamCapture struct {
	body io.ReadCloser
	buf  bytes.Buffer
}

// wrapTestUpstreamResponse 包装响应体，不改变读取行为；resp 为空时返回 nil。
func wrapTestUpstreamResponse(resp *http.Response) *testUpstreamCapture {
	if resp == nil || resp.Body == nil {
		return nil
	}
	capture := &testUpstreamCapture{body: resp.Body}
	resp.Body = capture
	return capture
}

func (c *testUpstreamCapture) Read(p []byte) (int, error) {
	n, err := c.body.Read(p)
	if n > 0 {
		if remaining := testUpstreamCaptureLimit - c.buf.Len(); remaining > 0 {
			c.buf.Write(p[:min(n, remaining)])
		}
	}
	return n, err
}

func (c *testUpstreamCapture) Close() error { return c.body.Close() }

func (c *testUpstreamCapture) Bytes() []byte {
	if c == nil {
		return nil
	}
	return c.buf.Bytes()
}

// extractTestUpstreamModel 从上游响应体提取模型名，支持 JSON 与 SSE 流。
func extractTestUpstreamModel(body []byte) string {
	if gjson.ValidBytes(body) {
		return extractTestModelJSON(body)
	}

	// 先按事件拼装 SSE data 字段再解析，网络分片、CRLF、多行事件与大事件
	// 都不影响提取。
	var event []byte
	for _, line := range bytes.Split(body, []byte{'\n'}) {
		line = bytes.TrimSuffix(line, []byte{'\r'})
		if len(line) == 0 {
			if name := extractTestModelJSON(event); name != "" {
				return name
			}
			event = event[:0]
			continue
		}
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		if len(event) > 0 {
			event = append(event, '\n')
		}
		event = append(event, bytes.TrimPrefix(line[len("data:"):], []byte{' '})...)
	}
	return extractTestModelJSON(event)
}

func extractTestModelJSON(body []byte) string {
	if !gjson.ValidBytes(body) {
		return ""
	}
	value := gjson.ParseBytes(body)
	if !value.IsArray() {
		return extractTestModelMetadata(value)
	}
	var name string
	value.ForEach(func(_, item gjson.Result) bool {
		name = extractTestModelMetadata(item)
		return name == ""
	})
	return name
}

func extractTestModelMetadata(value gjson.Result) string {
	if !value.IsObject() {
		return ""
	}
	// 只从响应元数据字段提取，不碰生成文本、工具参数与请求回显。
	for _, path := range []string{
		"model", "modelVersion", "response.model", "response.modelVersion",
		"message.model", "data.model", "result.model", "result.modelVersion",
	} {
		field := value.Get(path)
		if field.Type == gjson.String {
			if name := strings.TrimSpace(field.String()); name != "" {
				return name
			}
		}
	}
	return ""
}
