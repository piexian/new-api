package relay

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTypeSafeNativeRequestBodyPassthrough(t *testing.T) {
	t.Parallel()

	payload := `{"model":"jev-latest","state":{"messages":["hi"]},"questions":{"q":{"type":"noul","instructions":"ok?"}}}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{}

	reader, closer, err := typesafeNativeRequestBody(c, info)
	if closer != nil {
		defer closer.Close()
	}
	require.NoError(t, err)
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, payload, string(got))
	require.Equal(t, int64(len(payload)), info.UpstreamRequestBodySize)
}

func TestTypeSafeNativeRequestBodyAppliesParamOverride(t *testing.T) {
	t.Parallel()

	payload := `{"model":"jev-latest","state":"test","questions":{"q":{"type":"noul","instructions":"ok?"}}}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")

	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ParamOverride: map[string]any{"model": "jev-1.13.0"},
	}

	reader, closer, err := typesafeNativeRequestBody(c, info)
	if closer != nil {
		defer closer.Close()
	}
	require.NoError(t, err)
	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.JSONEq(t, `{"model":"jev-1.13.0","state":"test","questions":{"q":{"type":"noul","instructions":"ok?"}}}`, string(got))
}

func TestTypeSafeNativeHelperRejectsNonTypeSafeChannel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader("{}"))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)

	newAPIError := TypeSafeNativeHelper(c, &relaycommon.RelayInfo{})
	require.NotNil(t, newAPIError)
	require.Equal(t, http.StatusBadRequest, newAPIError.StatusCode)
	require.Contains(t, newAPIError.Error(), "native endpoint is not supported")
}

func TestPath2RelayModeSystemOne(t *testing.T) {
	t.Parallel()

	// /v1/systemone 必须归为 RelayModeTypeSafeNative, 否则会落入文本协议转换路径
	if got := relayconstant.Path2RelayMode("/v1/systemone"); got != relayconstant.RelayModeTypeSafeNative {
		t.Fatalf("Path2RelayMode(/v1/systemone) = %d, want RelayModeTypeSafeNative", got)
	}
}
