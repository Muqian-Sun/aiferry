package claude

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const cliVersionHelperEnv = "AIFERRY_TEST_CLI_VERSION_HELPER"

// 子进程入口：只在 TestCLIVersionEnvOnlyReadsAiFerryName 拉起时才输出，平时直接跳过。
func TestCLIVersionEnvHelperProcess(t *testing.T) {
	if os.Getenv(cliVersionHelperEnv) != "1" {
		t.Skip("仅作为 TestCLIVersionEnvOnlyReadsAiFerryName 的子进程运行")
	}
	fmt.Printf("CLIVERSION=%s\n", CLIVersion())
}

// 覆盖变量名是对运维的契约：只认 AIFERRY_CLAUDE_CLI_VERSION，旧名 SUB2API_CLAUDE_CLI_VERSION
// 不再生效。resolvedCLIVersion 在包初始化时就读了环境变量，进程内改不了，所以起子进程验证。
// 变量名用字面量，不用 CLIVersionEnv——否则常量改回旧名也照样通过。
func TestCLIVersionEnvOnlyReadsAiFerryName(t *testing.T) {
	const override = "999.0.0" // 纯数字三段、高于任何现实基线，IsSupportedCLIVersion 认可
	cases := []struct {
		name string
		env  []string
		want string
	}{
		{"新名生效", []string{"AIFERRY_CLAUDE_CLI_VERSION=" + override}, override},
		{"旧名不再生效", []string{"SUB2API_CLAUDE_CLI_VERSION=" + override}, CLICurrentVersion},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := make([]string, 0, len(os.Environ())+len(tc.env)+1)
			for _, kv := range os.Environ() {
				key, _, _ := strings.Cut(kv, "=")
				if key == "AIFERRY_CLAUDE_CLI_VERSION" || key == "SUB2API_CLAUDE_CLI_VERSION" || key == cliVersionHelperEnv {
					continue
				}
				env = append(env, kv)
			}
			env = append(env, cliVersionHelperEnv+"=1")
			env = append(env, tc.env...)

			cmd := exec.Command(os.Args[0], "-test.run=^TestCLIVersionEnvHelperProcess$", "-test.count=1") //nolint:gosec // G702: 重新执行当前测试二进制自身，参数都是常量
			cmd.Env = env
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("子进程失败: %v\n%s", err, out)
			}
			got := ""
			for _, line := range strings.Split(string(out), "\n") {
				if v, ok := strings.CutPrefix(line, "CLIVERSION="); ok {
					got = strings.TrimSpace(v)
				}
			}
			if got != tc.want {
				t.Fatalf("CLIVersion() = %q, want %q（子进程输出：\n%s）", got, tc.want, out)
			}
		})
	}
}

func TestIsSupportedCLIVersion(t *testing.T) {
	cases := []struct {
		name    string
		version string
		want    bool
	}{
		{"内置基线本身", CLICurrentVersion, true},
		{"高于基线的补丁位", "2.1.259", true},
		{"高于基线的次版本", "2.2.0", true},
		{"高于基线的主版本", "3.0.0", true},
		{"低于基线", "2.1.219", false},
		{"远低于基线", "1.0.0", false},
		{"空值", "", false},
		{"只有两段", "2.2", false},
		{"带 v 前缀", "v2.2.0", false},
		{"预发布后缀", "2.2.0-local", false},
		{"构建元数据", "2.2.0+build1", false},
		{"哨兵版本带后缀", "999.0.0-local", false},
		{"纯数字哨兵仍然合法（形态没问题，运维自负）", "999.0.0", true},
		{"非数字", "abc", false},
		{"多余的段", "2.2.0.1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsSupportedCLIVersion(tc.version); got != tc.want {
				t.Fatalf("IsSupportedCLIVersion(%q) = %v, want %v", tc.version, got, tc.want)
			}
		})
	}
}

func TestResolveCLIVersion(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"未配置时用内置基线", "", CLICurrentVersion},
		{"只有空白等同未配置", "   ", CLICurrentVersion},
		{"合法覆盖生效", "2.1.259", "2.1.259"},
		{"两侧空白被裁掉", "  2.1.259  ", "2.1.259"},
		{"非法值回落基线", "not-a-version", CLICurrentVersion},
		{"向下覆盖被拒", "2.0.0", CLICurrentVersion},
		{"预发布后缀被拒（会毒化账号指纹）", "2.2.0-local", CLICurrentVersion},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveCLIVersion(tc.raw); got != tc.want {
				t.Fatalf("resolveCLIVersion(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// 伪装身份的自洽性：User-Agent 头里的版本号必须与 CLIVersion() 一致。
// 两者由不同代码路径写入同一个请求，不一致会被上游判为非正版客户端。
func TestDefaultHeadersUserAgentMatchesCLIVersion(t *testing.T) {
	want := "claude-cli/" + CLIVersion() + " (external, cli)"
	if got := DefaultHeaders["User-Agent"]; got != want {
		t.Fatalf("DefaultHeaders[User-Agent] = %q, want %q", got, want)
	}
}

// 没有配置覆盖时，CLIVersion() 必须等于内置基线——保证本 PR 对既有部署零行为变化。
func TestCLIVersionDefaultsToBuiltinPin(t *testing.T) {
	if got := CLIVersion(); got != CLICurrentVersion {
		t.Fatalf("CLIVersion() = %q, want built-in pin %q (测试进程未设置 %s)", got, CLICurrentVersion, CLIVersionEnv)
	}
}
