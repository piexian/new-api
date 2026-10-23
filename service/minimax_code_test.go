package service

import (
	"testing"
	"time"
)

func TestNormalizeMiniMaxCodeRegion(t *testing.T) {
	cases := map[string]string{
		"":         "cn",
		"cn":       "cn",
		"CN":       "cn",
		"io":       "global",
		"global":   "global",
		"overseas": "global",
	}
	for input, want := range cases {
		if got := normalizeMiniMaxCodeRegion(input); got != want {
			t.Fatalf("normalizeMiniMaxCodeRegion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMiniMaxCodeAccountOrigins(t *testing.T) {
	if MiniMaxCodeAccountOrigin("cn") != "https://account.minimax.cn" {
		t.Fatal("unexpected cn oauth origin")
	}
	if MiniMaxCodeAccountOrigin("global") != "https://account.minimax.io" {
		t.Fatal("unexpected global oauth origin")
	}
	if MiniMaxCodeMatrixOrigin("cn") != minimaxCodeMatrixOriginCN {
		t.Fatal("unexpected cn matrix origin")
	}
	if MiniMaxCodeOpenPlatformOrigin("global") != minimaxCodeOpenOriginGlobal {
		t.Fatal("unexpected global open platform origin")
	}
}

func TestParseMiniMaxCodeTokenGrant(t *testing.T) {
	grant, err := parseMiniMaxCodeTokenGrant(map[string]any{
		"access_token":  "a.b.c",
		"refresh_token": "r2",
		"token_type":    "Bearer",
		"expires_in":    float64(3600),
	}, "r1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if grant.RefreshToken != "r2" || grant.AccessToken != "a.b.c" {
		t.Fatalf("unexpected grant: %+v", grant)
	}
	if grant.ExpiresAt.Before(time.Now()) {
		t.Fatal("expires_at should be in the future")
	}

	// refresh_token 缺省时保留旧值
	grant, err = parseMiniMaxCodeTokenGrant(map[string]any{
		"access_token": "a.b.c",
		"token_type":   "bearer",
		"expires_in":   float64(60),
	}, "r1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if grant.RefreshToken != "r1" {
		t.Fatalf("fallback refresh token not kept: %q", grant.RefreshToken)
	}

	// 非 Bearer / 缺字段应拒绝
	if _, err := parseMiniMaxCodeTokenGrant(map[string]any{
		"access_token":  "a.b.c",
		"refresh_token": "r",
		"token_type":    "MAC",
		"expires_in":    float64(60),
	}, ""); err == nil {
		t.Fatal("non-bearer token should be rejected")
	}
	if _, err := parseMiniMaxCodeTokenGrant(map[string]any{
		"access_token":  "a.b.c",
		"refresh_token": "r",
		"expires_in":    float64(0),
	}, ""); err == nil {
		t.Fatal("missing expires_in should be rejected")
	}
}

func TestMiniMaxCodeSignedHeaders(t *testing.T) {
	// 已知输入的 md5 校验：验证拼接顺序（盐在时间戳后、body 结尾）。
	nowMs := int64(1700000000123)
	path := "/v1/api/user/info?a=1"
	body := []byte(`{"k":"v"}`)
	header := miniMaxCodeSignedHeaders(nowMs, path, body)
	if got := header.Get("x-timestamp"); got != "1700000000" {
		t.Fatalf("x-timestamp = %q", got)
	}
	wantSig := md5Hex("1700000000" + minimaxCodeSignatureSalt + string(body))
	if header.Get("x-signature") != wantSig {
		t.Fatalf("x-signature = %q, want %q", header.Get("x-signature"), wantSig)
	}
	wantYY := md5Hex(encodeURIComponent(path) + "_" + string(body) + md5Hex("1700000000123") + minimaxCodeYYSuffix)
	if header.Get("yy") != wantYY {
		t.Fatalf("yy = %q, want %q", header.Get("yy"), wantYY)
	}

	// 空 body 按 '{}' 处理
	empty := miniMaxCodeSignedHeaders(nowMs, path, nil)
	if empty.Get("Content-Type") == "" {
		t.Fatal("Content-Type missing")
	}
	wantYYEmpty := md5Hex(encodeURIComponent(path) + "_{}" + md5Hex("1700000000123") + minimaxCodeYYSuffix)
	if empty.Get("yy") != wantYYEmpty {
		t.Fatalf("yy(empty) = %q, want %q", empty.Get("yy"), wantYYEmpty)
	}
}

func TestEncodeURIComponent(t *testing.T) {
	if got := encodeURIComponent("/a b/c?x=1&y=中"); got != "/a%20b/c%3Fx%3D1%26y%3D%E4%B8%AD" {
		t.Fatalf("unexpected encoding: %q", got)
	}
	if got := encodeURIComponent("a.b~c*d'e(f)g-h_i!j"); got != "a.b~c*d'e(f)g-h_i!j" {
		t.Fatalf("unreserved chars should stay: %q", got)
	}
}

func TestParseMiniMaxCodeOAuthKey(t *testing.T) {
	if _, err := ParseMiniMaxCodeOAuthKey(""); err == nil {
		t.Fatal("empty key should be rejected")
	}
	key, err := ParseMiniMaxCodeOAuthKey(`{"access_token":"a","refresh_token":"r","region":"global","group_id":"g"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.Region != "global" || key.GroupID != "g" {
		t.Fatalf("unexpected key: %+v", key)
	}
	encoded, err := key.Marshal()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if _, err := ParseMiniMaxCodeOAuthKey(encoded); err != nil {
		t.Fatalf("re-parse failed: %v", err)
	}
}
