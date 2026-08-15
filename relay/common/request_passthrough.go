package common

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/model_setting"

	"github.com/gin-gonic/gin"
)

func IsRequestPassThroughEnabled(info *RelayInfo) bool {
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled {
		return true
	}
	return info != nil && info.ChannelMeta != nil && info.ChannelSetting.PassThroughBodyEnabled
}

// PassThroughRequestBody 读取透传请求体；发生模型映射时把 body 里的 model
// 改写为上游模型名，避免透传原始模型 ID 导致上游报"模型不存在"。
// 非 JSON body（如 multipart）或无 model 字段时原样透传。
func PassThroughRequestBody(c *gin.Context, info *RelayInfo) (io.Reader, int64, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, 0, err
	}
	mapped := info != nil && info.ChannelMeta != nil &&
		info.IsModelMapped && info.UpstreamModelName != ""
	if !mapped {
		return common.ReaderOnly(storage), storage.Size(), nil
	}
	raw, err := storage.Bytes()
	if err != nil {
		return nil, 0, err
	}
	var body map[string]json.RawMessage
	if err := common.Unmarshal(raw, &body); err != nil {
		return bytes.NewReader(raw), int64(len(raw)), nil
	}
	if _, ok := body["model"]; !ok {
		return bytes.NewReader(raw), int64(len(raw)), nil
	}
	modelJSON, err := common.Marshal(info.UpstreamModelName)
	if err != nil {
		return nil, 0, err
	}
	body["model"] = json.RawMessage(modelJSON)
	rewritten, err := common.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	return bytes.NewReader(rewritten), int64(len(rewritten)), nil
}
