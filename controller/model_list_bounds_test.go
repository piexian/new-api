package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"

	"github.com/stretchr/testify/assert"
)

// TestAnthropicListBounds 覆盖 /v1/models（Anthropic 形态）空列表场景：
// 旧实现对空切片取 [0] 会 panic（生产日志持续可见），现返回 null 游标。
func TestAnthropicListBounds(t *testing.T) {
	t.Run("empty list returns null cursors", func(t *testing.T) {
		firstID, lastID := anthropicListBounds(nil)
		assert.Nil(t, firstID)
		assert.Nil(t, lastID)

		firstID, lastID = anthropicListBounds([]dto.AnthropicModel{})
		assert.Nil(t, firstID)
		assert.Nil(t, lastID)
	})

	t.Run("populated list returns first and last ids", func(t *testing.T) {
		models := []dto.AnthropicModel{
			{ID: "claude-opus-4-1"},
			{ID: "claude-sonnet-4-5"},
			{ID: "claude-haiku-4-5"},
		}
		firstID, lastID := anthropicListBounds(models)
		assert.Equal(t, "claude-opus-4-1", firstID)
		assert.Equal(t, "claude-haiku-4-5", lastID)
	})
}
