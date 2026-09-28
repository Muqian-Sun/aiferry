//go:build unit

package service

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func newGatewayServiceForDebugEnvTest() *GatewayService {
	return NewGatewayService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}

// 网关调试开关只认 AIFERRY_ 前缀的环境变量，旧名 SUB2API_ 不再生效（不做兼容）。
// 变量名用字面量，不用 debugGatewayBodyEnv——否则常量改回旧名也照样通过。
func TestNewGatewayServiceDebugEnvOnlyReadsAiFerryNames(t *testing.T) {
	t.Run("新名生效", func(t *testing.T) {
		logPath := filepath.Join(t.TempDir(), "gateway_debug.log")
		t.Setenv("SUB2API_DEBUG_CLAUDE_MIMIC", "")
		t.Setenv("SUB2API_DEBUG_GATEWAY_BODY", "")
		t.Setenv("AIFERRY_DEBUG_CLAUDE_MIMIC", "1")
		t.Setenv("AIFERRY_DEBUG_GATEWAY_BODY", logPath)

		svc := newGatewayServiceForDebugEnvTest()
		f := svc.debugGatewayBodyFile.Load()
		require.NotNil(t, f)
		t.Cleanup(func() { _ = f.Close() })

		require.True(t, svc.debugClaudeMimicEnabled())
		require.Equal(t, logPath, f.Name())
		require.FileExists(t, logPath)
	})

	t.Run("旧名不再生效", func(t *testing.T) {
		logPath := filepath.Join(t.TempDir(), "gateway_debug.log")
		t.Setenv("AIFERRY_DEBUG_CLAUDE_MIMIC", "")
		t.Setenv("AIFERRY_DEBUG_GATEWAY_BODY", "")
		t.Setenv("SUB2API_DEBUG_CLAUDE_MIMIC", "1")
		t.Setenv("SUB2API_DEBUG_GATEWAY_BODY", logPath)

		svc := newGatewayServiceForDebugEnvTest()
		if f := svc.debugGatewayBodyFile.Load(); f != nil {
			_ = f.Close()
			t.Fatalf("旧名 SUB2API_DEBUG_GATEWAY_BODY 不应再打开调试日志，却打开了 %s", f.Name())
		}
		require.False(t, svc.debugClaudeMimicEnabled())
		require.NoFileExists(t, logPath)
	})
}
