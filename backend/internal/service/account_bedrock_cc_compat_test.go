//go:build unit

package service

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// 账号 extra.bedrock_cc_compat 是开关的唯一来源；只认 bool true。
func TestBedrockCCCompatEnabled_ExtraValues(t *testing.T) {
	tests := []struct {
		name  string
		extra map[string]any
		want  bool
	}{
		{name: "bool true", extra: map[string]any{featureKeyBedrockCCCompat: true}, want: true},
		{name: "bool false", extra: map[string]any{featureKeyBedrockCCCompat: false}, want: false},
		{name: "string yes", extra: map[string]any{featureKeyBedrockCCCompat: "yes"}, want: false},
		{name: "old map format", extra: map[string]any{featureKeyBedrockCCCompat: map[string]any{"bedrock": true}}, want: false},
		{name: "missing key", extra: map[string]any{"other_feature": true}, want: false},
		{name: "nil extra", extra: nil, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Extra: tt.extra}
			require.Equal(t, tt.want, a.BedrockCCCompatEnabled())
		})
	}
}

func TestBedrockCCCompatEnabled_NilAccount(t *testing.T) {
	var a *Account
	require.False(t, a.BedrockCCCompatEnabled())
}

// ApplyBedrockCCCompat 只看账号开关：开了才清 CC 专有字段并补 anthropic_version，关了 body 原样返回。
func TestApplyBedrockCCCompat_FollowsAccountSwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"claude-sonnet-4-5","service_tier":"auto","max_tokens":10,"messages":[{"role":"user","content":"hi"}]}`)
	newCtx := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/v1/messages", nil)
		return c
	}
	svc := &GatewayService{}

	off := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Extra: map[string]any{featureKeyBedrockCCCompat: false}}
	gotOff := svc.ApplyBedrockCCCompat(newCtx(), body, "claude-sonnet-4-5", off)
	require.Equal(t, string(body), string(gotOff))

	on := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Extra: map[string]any{featureKeyBedrockCCCompat: true}}
	gotOn := svc.ApplyBedrockCCCompat(newCtx(), body, "claude-sonnet-4-5", on)
	require.False(t, gjson.GetBytes(gotOn, "service_tier").Exists(), "CC-only field must be stripped")
	require.Equal(t, "bedrock-2023-05-31", gjson.GetBytes(gotOn, "anthropic_version").String())
}
