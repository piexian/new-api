package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseQwenTokenPlanUsageResponse(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"code":"200",
		"data":{
			"DataV2":{
				"ret":["SUCCESS::接口调用成功"],
				"data":{
					"msg":"Success.",
					"code":"SUCCESS",
					"data":{
						"per5HourPercentage":0.32,
						"per5HourResetTime":1788364680000,
						"per1WeekPercentage":1.0,
						"per1WeekResetTime":1788364680000
					},
					"requestId":"req-1",
					"success":true
				}
			},
			"success":true,
			"httpStatus":200,
			"errorCode":"",
			"api":"zeldaHttp.apikeyMgr./tokenplan/personal/api/v2/usage",
			"errorMsg":""
		},
		"httpStatusCode":"200",
		"requestId":"req-1",
		"successResponse":true
	}`)

	usage, notLogined, success, message := parseQwenTokenPlanUsageResponse(http.StatusOK, body)
	require.True(t, success)
	require.Empty(t, message)
	require.False(t, notLogined)
	require.True(t, usage.Subscribed)
	require.True(t, usage.Per5Hour.Present)
	require.InDelta(t, 32, usage.Per5Hour.UsedPercent, 0.001)
	require.EqualValues(t, 1788364680000, usage.Per5Hour.ResetAt)
	require.True(t, usage.Per1Week.Present)
	require.InDelta(t, 100, usage.Per1Week.UsedPercent, 0.001)
}

func TestParseQwenTokenPlanUsageResponseWeeklyOnly(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"code":"200",
		"data":{
			"DataV2":{"data":{"success":true,"data":{"per1WeekPercentage":0.05,"per1WeekResetTime":1788364680000}}},
			"success":true,"httpStatus":200,"errorCode":"","errorMsg":""
		},
		"successResponse":true
	}`)

	usage, _, success, _ := parseQwenTokenPlanUsageResponse(http.StatusOK, body)
	require.True(t, success)
	require.True(t, usage.Subscribed)
	require.False(t, usage.Per5Hour.Present)
	require.Zero(t, usage.Per5Hour.UsedPercent)
	require.True(t, usage.Per1Week.Present)
	require.InDelta(t, 5, usage.Per1Week.UsedPercent, 0.001)
}

func TestParseQwenTokenPlanUsageResponseNotLogined(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"code":"200",
		"data":{
			"success":false,
			"httpStatus":200,
			"errorCode":"BailianGateway.Login.NotLogined",
			"api":"zeldaHttp.apikeyMgr./tokenplan/personal/api/v2/usage",
			"errorMsg":"BailianGateway.Login.NotLogined"
		},
		"successResponse":true
	}`)

	usage, notLogined, success, message := parseQwenTokenPlanUsageResponse(http.StatusOK, body)
	require.False(t, success)
	require.True(t, notLogined)
	require.NotEmpty(t, message)
	require.False(t, usage.Subscribed)
}

func TestParseQwenTokenPlanUsageResponseUpstreamError(t *testing.T) {
	t.Parallel()

	_, notLogined, success, message := parseQwenTokenPlanUsageResponse(http.StatusForbidden, []byte(`{}`))
	require.False(t, success)
	require.False(t, notLogined)
	require.Contains(t, message, "403")

	_, _, success, message = parseQwenTokenPlanUsageResponse(http.StatusOK, []byte(`not-json`))
	require.False(t, success)
	require.NotEmpty(t, message)

	body := []byte(`{"data":{"success":false,"errorCode":"Some.Other","errorMsg":"boom"}}`)
	_, notLogined, success, message = parseQwenTokenPlanUsageResponse(http.StatusOK, body)
	require.False(t, success)
	require.False(t, notLogined)
	require.Equal(t, "boom", message)
}

func TestDoQwenTokenPlanUsageRequest(t *testing.T) {
	t.Parallel()

	var gotAuth, gotContentType, gotAccept, gotMethod, gotAPI, gotParams, gotRegion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotAccept = r.Header.Get("Accept")
		gotAPI = r.URL.Query().Get("api")
		require.NoError(t, r.ParseForm())
		gotParams = r.PostFormValue("params")
		gotRegion = r.PostFormValue("region")
		fmt.Fprint(w, `{"code":"200","data":{"success":true,"DataV2":{"data":{"success":true,"data":{}}}}}`)
	}))
	defer server.Close()

	original := qwenTokenPlanGatewayURL
	qwenTokenPlanGatewayURL = server.URL + "/cli/api.json"
	defer func() { qwenTokenPlanGatewayURL = original }()

	statusCode, body, err := doQwenTokenPlanUsageRequest(context.Background(), server.Client(), "ct-token")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, statusCode)
	require.Equal(t, http.MethodPost, gotMethod)
	require.Equal(t, "Bearer ct-token", gotAuth)
	require.Equal(t, "application/x-www-form-urlencoded", gotContentType)
	require.Equal(t, "*/*", gotAccept)
	require.Equal(t, qwenTokenPlanUsageAPI, gotAPI)
	require.Equal(t, qwenTokenPlanConsoleRegion, gotRegion)
	require.Contains(t, gotParams, qwenTokenPlanUsageAPI)
	require.Contains(t, gotParams, "cornerstoneParam")
	require.Contains(t, gotParams, "BAILIAN_ALIYUN")
	require.Contains(t, string(body), `"success":true`)
}

func TestGenerateQwenConsoleToken(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NotEmpty(t, r.Header.Get("authorization"))
		require.Contains(t, r.Header.Get("authorization"), "ACS3-HMAC-SHA256 Credential=test-ak")
		require.Equal(t, qwenTokenPlanTokenAction, r.Header.Get("x-acs-action"))
		require.Equal(t, qwenTokenPlanTokenVersion, r.Header.Get("x-acs-version"))
		require.NotEmpty(t, r.Header.Get("x-acs-signature-nonce"))
		require.NotEmpty(t, r.Header.Get("x-acs-content-sha256"))
		fmt.Fprint(w, `{"cliAccessToken":"fresh-token"}`)
	}))
	defer server.Close()

	original := qwenTokenPlanTokenURL
	qwenTokenPlanTokenURL = server.URL + qwenTokenPlanTokenPath
	defer func() { qwenTokenPlanTokenURL = original }()

	token, err := generateQwenConsoleToken(context.Background(), server.Client(), "test-ak", "test-sk")
	require.NoError(t, err)
	require.Equal(t, "fresh-token", token)
}

func TestQwenAcs3SignHeadersStructure(t *testing.T) {
	t.Parallel()

	headers := qwenAcs3SignHeaders(http.MethodPost, qwenTokenPlanTokenHost, qwenTokenPlanTokenPath, "",
		qwenTokenPlanTokenAction, qwenTokenPlanTokenVersion, "ak", "sk", "", []byte{})

	require.Equal(t, qwenTokenPlanTokenHost, headers["host"])
	require.Equal(t, "application/json", headers["content-type"])

	authorization := headers["authorization"]
	require.True(t, strings.HasPrefix(authorization, "ACS3-HMAC-SHA256 Credential=ak,SignedHeaders="))

	parts := strings.SplitN(strings.TrimPrefix(authorization, "ACS3-HMAC-SHA256 "), ",", 3)
	require.Len(t, parts, 3)
	require.Equal(t, "Credential=ak", parts[0])
	signedHeaders := strings.TrimPrefix(parts[1], "SignedHeaders=")
	require.Equal(t, "content-type;host;x-acs-action;x-acs-content-sha256;x-acs-date;x-acs-signature-nonce;x-acs-version", signedHeaders)
	require.True(t, strings.HasPrefix(parts[2], "Signature="))
	require.Len(t, strings.TrimPrefix(parts[2], "Signature="), 64)
}

// TestQwenAcs3SignMatchesReference 用 node 按 bailian-cli 原始签名算法生成的期望值做跨实现校验。
func TestQwenAcs3SignMatchesReference(t *testing.T) {
	t.Parallel()

	headers := qwenAcs3Sign(http.MethodPost, qwenTokenPlanTokenHost, qwenTokenPlanTokenPath, "",
		qwenTokenPlanTokenAction, qwenTokenPlanTokenVersion, "test-ak", "test-sk", "", []byte{},
		"2026-09-02T12:00:00Z", "fixed-nonce-1")

	require.Equal(t, "2026-09-02T12:00:00Z", headers["x-acs-date"])
	require.Equal(t, "fixed-nonce-1", headers["x-acs-signature-nonce"])
	require.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", headers["x-acs-content-sha256"])
	require.Equal(t,
		"ACS3-HMAC-SHA256 Credential=test-ak,SignedHeaders=content-type;host;x-acs-action;x-acs-content-sha256;x-acs-date;x-acs-signature-nonce;x-acs-version,Signature=b87189401f9fff610691c963856f0a23315a4f1390a4b0662038390b387611a4",
		headers["authorization"])
}
