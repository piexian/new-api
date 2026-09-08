package service

import (
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInlineMediaCacheDistinguishesEqualLengthSharedPrefixes(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	t.Cleanup(func() { CleanupFileSources(c) })
	prefix := strings.Repeat("shared media bytes", 20)
	first := base64.StdEncoding.EncodeToString([]byte(prefix + "A"))
	second := base64.StdEncoding.EncodeToString([]byte(prefix + "B"))
	require.Equal(t, len(first), len(second))
	require.Equal(t, first[:128], second[:128])
	got, _, err := GetBase64Data(c, types.NewBase64FileSource(first, "application/pdf"))
	require.NoError(t, err)
	require.Equal(t, first, got)
	got, _, err = GetBase64Data(c, types.NewBase64FileSource(second, "application/pdf"))
	require.NoError(t, err)
	require.Equal(t, second, got)
}
