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
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSystemOneBodyMappingPreservesNativeFields(t *testing.T) {
	payload := `{"model":"alias", "state":{"flag":false,"n":0}, "questions":{"q":{"type":"choice","criteria":{"a":"A"}}}, "extra":null}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{IsModelMapped: true, UpstreamModelName: "jev-1.13-free"}}
	reader, closer, err := typesafeNativeRequestBody(c, info)
	if closer != nil {
		defer closer.Close()
	}
	require.NoError(t, err)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, strings.Replace(payload, `"alias"`, `"jev-1.13-free"`, 1), string(body))
}

func TestSystemOneOpenCodeHelperReachesNativeUpstream(t *testing.T) {
	service.InitHttpClient()
	got := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- r.URL.Path + "\n" + string(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"test upstream error","type":"invalid_request_error"}}`))
	}))
	defer upstream.Close()
	payload := `{"model":"alias","state":"test","questions":{"q":{"type":"noul"}}}`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenCode)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL+"/zen")
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "alias")
	c.Set("model_mapping", `{"alias":"jev-1.13-free"}`)
	info := &relaycommon.RelayInfo{OriginModelName: "alias", RelayFormat: types.RelayFormatTypeSafe, RelayMode: relayconstant.RelayModeTypeSafeNative}
	apiErr := TypeSafeNativeHelper(c, info)
	require.NotNil(t, apiErr)
	require.Contains(t, apiErr.Error(), "test upstream error")
	require.Equal(t, "/zen/v1/systemone\n"+strings.Replace(payload, `"alias"`, `"jev-1.13-free"`, 1), <-got)
}
