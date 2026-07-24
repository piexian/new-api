package model

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestTruncateEmailLogContentPreservesUTF8(t *testing.T) {
	content := strings.Repeat("邮", maxEmailLogContentBytes)
	truncated := truncateEmailLogContent(content)
	require.LessOrEqual(t, len(truncated), maxEmailLogContentBytes)
	require.True(t, utf8.ValidString(truncated))
	require.Equal(t, "<p>short email</p>", truncateEmailLogContent("<p>short email</p>"))
}
