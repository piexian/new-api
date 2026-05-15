package dto

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type MiniMaxMusicGenerationRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt,omitempty"`
	Lyrics         string `json:"lyrics,omitempty"`
	Stream         bool   `json:"stream,omitempty"`
	OutputFormat   string `json:"output_format,omitempty"`
	AudioURL       string `json:"audio_url,omitempty"`
	AudioBase64    string `json:"audio_base64,omitempty"`
	CoverFeatureID string `json:"cover_feature_id,omitempty"`
}

func (r *MiniMaxMusicGenerationRequest) GetTokenCountMeta() *types.TokenCountMeta {
	return &types.TokenCountMeta{
		CombineText: strings.TrimSpace(strings.Join([]string{r.Prompt, r.Lyrics}, "\n")),
		TokenType:   types.TokenTypeTextNumber,
	}
}

func (r *MiniMaxMusicGenerationRequest) IsStream(c *gin.Context) bool {
	return r.Stream
}

func (r *MiniMaxMusicGenerationRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}

func (r *MiniMaxMusicGenerationRequest) GetLogContent() []string {
	logContent := make([]string, 0, 3)
	if prompt := strings.TrimSpace(r.Prompt); prompt != "" {
		logContent = append(logContent, fmt.Sprintf("风格 %s", prompt))
	}
	if outputFormat := strings.TrimSpace(r.OutputFormat); outputFormat != "" {
		logContent = append(logContent, fmt.Sprintf("输出格式 %s", outputFormat))
	}
	if r.CoverFeatureID != "" || r.AudioURL != "" || r.AudioBase64 != "" {
		logContent = append(logContent, "翻唱生成")
	}
	return logContent
}

type MiniMaxMusicCoverPreprocessRequest struct {
	Model       string `json:"model"`
	AudioURL    string `json:"audio_url,omitempty"`
	AudioBase64 string `json:"audio_base64,omitempty"`
}

func (r *MiniMaxMusicCoverPreprocessRequest) GetTokenCountMeta() *types.TokenCountMeta {
	return &types.TokenCountMeta{
		CombineText: strings.TrimSpace(r.AudioURL),
		TokenType:   types.TokenTypeTextNumber,
	}
}

func (r *MiniMaxMusicCoverPreprocessRequest) IsStream(c *gin.Context) bool {
	return false
}

func (r *MiniMaxMusicCoverPreprocessRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}

func (r *MiniMaxMusicCoverPreprocessRequest) GetLogContent() []string {
	return []string{"翻唱前处理"}
}

type MiniMaxLyricsGenerationRequest struct {
	Mode   string `json:"mode"`
	Prompt string `json:"prompt,omitempty"`
	Lyrics string `json:"lyrics,omitempty"`
	Title  string `json:"title,omitempty"`
}

func (r *MiniMaxLyricsGenerationRequest) GetTokenCountMeta() *types.TokenCountMeta {
	return &types.TokenCountMeta{
		CombineText: strings.TrimSpace(strings.Join([]string{r.Prompt, r.Lyrics, r.Title}, "\n")),
		TokenType:   types.TokenTypeTextNumber,
	}
}

func (r *MiniMaxLyricsGenerationRequest) IsStream(c *gin.Context) bool {
	return false
}

func (r *MiniMaxLyricsGenerationRequest) SetModelName(modelName string) {
}

func (r *MiniMaxLyricsGenerationRequest) GetLogContent() []string {
	logContent := make([]string, 0, 2)
	if r.Mode != "" {
		logContent = append(logContent, fmt.Sprintf("模式 %s", r.Mode))
	}
	if r.Title != "" {
		logContent = append(logContent, fmt.Sprintf("标题 %s", r.Title))
	}
	return logContent
}
