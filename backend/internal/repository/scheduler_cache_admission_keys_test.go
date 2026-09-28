package repository

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 选号阶段（candidateAdmits、RPM、Compact 分级……）在快照的 meta 投影上判断，选中后才补全账号。
// 投影前后这些判断必须一致，否则快照命中时配置静默失效：RPM 不限、「只改名」映射变成白名单、
// 能力集不限、Compact 手动开关无效。
func TestSchedulerMetadataAccountKeepsAdmissionInputs(t *testing.T) {
	openai := service.Account{
		ID:          41,
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeOAuth,
		Status:      service.StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"model_mapping":             map[string]any{"my-alias": "gpt-5.4"},
			"model_mapping_rename_only": true,
			"openai_capabilities":       []any{"responses"},
			"auth_mode":                 service.OpenAIAuthModeAgentIdentity,
		},
		Extra: map[string]any{
			"base_rpm":                 12,
			"openai_compact_mode":      service.OpenAICompactModeForceOff,
			"openai_compact_supported": true,
		},
	}
	view := func(a *service.Account) map[string]any {
		compactSupported, compactKnown := a.OpenAICompactSupportKnown()
		return map[string]any{
			"base_rpm":          a.GetBaseRPM(),
			"rpm_sticky_buffer": a.GetRPMStickyBuffer(),
			"rpm_at_limit":      a.CheckRPMSchedulability(12),
			"unmapped_model":    a.IsModelSupported("gpt-5.4-mini"),
			"chat":              a.SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilityChatCompletions),
			"live":              a.SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilityLive),
			"compact_supported": compactSupported,
			"compact_known":     compactKnown,
		}
	}
	meta := buildSchedulerMetadataAccount(openai)
	full := view(&openai)
	// 先确认完整账号上这些配置真的起作用，差分比较才有意义
	require.Equal(t, 12, full["base_rpm"])
	require.Equal(t, true, full["unmapped_model"], "只改名的映射不兼任白名单")
	require.Equal(t, false, full["chat"], "能力集里没有 chat")
	require.Equal(t, false, full["live"], "agent identity 不承接 live")
	require.Equal(t, true, full["compact_known"])
	require.Equal(t, full, view(&meta))

	bedrock := service.Account{
		ID:          42,
		Platform:    service.PlatformAnthropic,
		Type:        service.AccountTypeBedrock,
		Status:      service.StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"aws_region": "eu-west-1"},
	}
	bedrockMeta := buildSchedulerMetadataAccount(bedrock)
	fullID, fullOK := service.ResolveBedrockModelID(&bedrock, "claude-sonnet-4-5")
	metaID, metaOK := service.ResolveBedrockModelID(&bedrockMeta, "claude-sonnet-4-5")
	require.True(t, fullOK)
	require.True(t, strings.HasPrefix(fullID, "eu."), fullID)
	require.Equal(t, fullID, metaID)
	require.Equal(t, fullOK, metaOK)

	grok := service.Account{
		ID:          43,
		Platform:    service.PlatformGrok,
		Type:        service.AccountTypeOAuth,
		Status:      service.StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"subscription_tier": "free", "team_id": "team-1"},
		// 计费快照在场但不完整：免费档只能靠 credentials.subscription_tier 认出来
		Extra: map[string]any{"grok_billing_snapshot": map[string]any{"status_code": 200, "partial": true}},
	}
	grokMeta := buildSchedulerMetadataAccount(grok)
	media := service.OpenAIEndpointCapabilityGrokMediaGeneration
	require.False(t, grok.SupportsOpenAIEndpointCapability(media), "免费档不接生图")
	require.Equal(t, grok.SupportsOpenAIEndpointCapability(media), grokMeta.SupportsOpenAIEndpointCapability(media))
	require.Equal(t, "team-1", grokMeta.GetCredential("team_id"), "团队级模型限流按 team_id 判")
}

// 快照白名单的守卫：用 AST 从所有读快照列表的入口出发，沿调用走到 Account 的方法、参数带 *Account 的函数
// 与调度文件里的函数（选中后的 hydrateSelectedAccount 不往下走），收集读到的 Extra / Credentials 键，
// 不在 filterSchedulerCredentials / filterSchedulerExtra 里的就失败。新加的调度判断读了新键、忘了进投影，这里会红。
// 调用图按名字建（偏多不偏少）；下标不是字面量 / 常量的一律报出来，逐个说明理由才放行。
func TestSchedulerMetadataWhitelistCoversAdmissionPath(t *testing.T) {
	// 有意不进投影的键：键 → 理由
	exempt := map[string]string{
		"credentials.access_token": "Grok 档位的最新信号（JWT 里的 tier）；另有 subscription_tier 与 grok_billing_snapshot，整段 token 不进每次选号都读的 meta",
	}
	// 下标不是字面量的函数：函数名 → 理由
	dynamicOK := map[string]string{
		"GetCredential":                         "通用取值函数，带字面量键的调用处已逐个统计；AccountService.GetCredential 与它同名",
		"ResolveOpenAIResponsesWebSocketV2Mode": "键以字面量传给内部闭包，WS 相关键都在白名单",
	}

	scan := newAdmissionScan(t, filepath.Join("..", "service"))
	roots := scan.callersOf("ListSchedulableAccounts", "listSchedulableAccounts", "listSchedulableAccountsOnce")
	require.NotEmpty(t, roots)
	reach := scan.reach(roots, map[string]bool{"hydrateSelectedAccount": true})
	require.True(t, reach["candidateAdmits"], "扫描应走到候选准入")

	whitelist := map[string]map[string]bool{
		"credentials": sliceSet(filterSchedulerCredentialsKeys(t)),
		"extra":       sliceSet(filterSchedulerExtraKeys(t)),
	}
	var problems []string
	for _, rd := range scan.reads {
		if !reach[rd.fn] || whitelist[rd.kind][rd.key] {
			continue
		}
		if _, ok := exempt[rd.kind+"."+rd.key]; ok {
			continue
		}
		problems = append(problems, fmt.Sprintf("%s.%s（%s，%s）不在快照白名单", rd.kind, rd.key, rd.fn, rd.pos))
	}
	for _, u := range scan.dynamic {
		if reach[u.fn] {
			if _, ok := dynamicOK[u.fn]; !ok {
				problems = append(problems, fmt.Sprintf("%s 在 %s 用非字面量取键，确认后加进 dynamicOK", u.fn, u.pos))
			}
		}
	}
	sort.Strings(problems)
	require.Empty(t, problems)
}

func sliceSet(keys []string) map[string]bool {
	out := make(map[string]bool, len(keys))
	for _, k := range keys {
		out[k] = true
	}
	return out
}

// 白名单从 filter 函数的实际行为取：给一份含所有候选键的 map，看哪些留下来。
func filterSchedulerCredentialsKeys(t *testing.T) []string {
	return keptKeys(t, "filterSchedulerCredentials", func(m map[string]any) map[string]any { return filterSchedulerCredentials(m) })
}

func filterSchedulerExtraKeys(t *testing.T) []string {
	return keptKeys(t, "filterSchedulerExtra", func(m map[string]any) map[string]any { return filterSchedulerExtra(m) })
}

func keptKeys(t *testing.T, fn string, filter func(map[string]any) map[string]any) []string {
	// 候选键 = scheduler_cache.go 里该函数体内出现的全部字符串字面量与 service 常量
	f, err := parser.ParseFile(token.NewFileSet(), "scheduler_cache.go", nil, 0)
	require.NoError(t, err)
	candidates := map[string]any{}
	found := false
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != fn {
			continue
		}
		found = true
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if v, err := strconv.Unquote(lit.Value); err == nil && v != "" {
					candidates[v] = "x"
				}
			}
			return true
		})
	}
	require.True(t, found, fn)
	// service 常量形式的键（UpstreamBillingProbeExtraKey 等）按值补进候选
	candidates[service.GrokMediaEligibleExtraKey] = true
	candidates[service.UpstreamBillingProbeExtraKey] = map[string]any{"status": "ok"} // 探测结果要有 status 才进投影
	kept := filter(candidates)
	out := make([]string, 0, len(kept))
	for k := range kept {
		out = append(out, k)
	}
	return out
}

type admissionRead struct{ kind, key, fn, pos string }

type admissionScan struct {
	fset      *token.FileSet
	consts    map[string]string
	calls     map[string]map[string]bool
	enterable map[string]bool
	helpers   map[string]string
	reads     []admissionRead
	dynamic   []admissionRead
}

var admissionSchedFiles = map[string]bool{
	"gateway_scheduling.go": true, "openai_gateway_scheduling.go": true, "scheduling_select_options.go": true,
	"account_scheduling_state.go": true, "scheduling_platform_eligibility.go": true,
	"account_scheduling_threshold_eval.go": true, "account_scheduling_threshold_reason.go": true, "temp_unsched.go": true,
}

var admissionKeyMethods = map[string]string{
	"GetCredential": "credentials", "GetCredentialAsTime": "credentials", "GetCredentialAsInt64": "credentials",
	"GetExtraString": "extra", "getExtraTime": "extra", "getExtraBool": "extra", "getExtraString": "extra", "getExtraInt": "extra",
}

func newAdmissionScan(t *testing.T, dir string) *admissionScan {
	s := &admissionScan{
		fset: token.NewFileSet(), consts: map[string]string{}, calls: map[string]map[string]bool{},
		enterable: map[string]bool{}, helpers: map[string]string{},
	}
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var files []*ast.File
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(s.fset, filepath.Join(dir, name), nil, 0)
		require.NoError(t, err, name)
		files = append(files, f)
	}
	require.NotEmpty(t, files)
	for _, f := range files {
		s.collectDecls(f)
	}
	for _, f := range files {
		s.collectBodies(f, filepath.Base(s.fset.Position(f.Pos()).Filename))
	}
	return s
}

func admissionIsAccount(e ast.Expr) bool {
	if st, ok := e.(*ast.StarExpr); ok {
		e = st.X
	}
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "Account"
}

func (s *admissionScan) collectDecls(f *ast.File) {
	for _, d := range f.Decls {
		switch x := d.(type) {
		case *ast.GenDecl:
			if x.Tok != token.CONST {
				continue
			}
			for _, sp := range x.Specs {
				vs := sp.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if i < len(vs.Values) {
						if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							v, _ := strconv.Unquote(lit.Value)
							s.consts[n.Name] = v
						}
					}
				}
			}
		case *ast.FuncDecl:
			// 首参是 map[string]any 且参数名含 extra / cred 的 helper
			if x.Recv != nil || x.Type.Params == nil || len(x.Type.Params.List) == 0 {
				continue
			}
			first := x.Type.Params.List[0]
			mt, ok := first.Type.(*ast.MapType)
			if !ok || len(first.Names) == 0 {
				continue
			}
			if k, ok := mt.Key.(*ast.Ident); !ok || k.Name != "string" {
				continue
			}
			pn := strings.ToLower(first.Names[0].Name)
			switch {
			case strings.Contains(pn, "extra"):
				s.helpers[x.Name.Name] = "extra"
			case strings.Contains(pn, "cred"):
				s.helpers[x.Name.Name] = "credentials"
			}
		}
	}
}

func (s *admissionScan) strOf(e ast.Expr) (string, bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			v, err := strconv.Unquote(x.Value)
			return v, err == nil
		}
	case *ast.Ident:
		v, ok := s.consts[x.Name]
		return v, ok
	}
	return "", false
}

func admissionFieldKind(e ast.Expr) string {
	name := ""
	switch x := e.(type) {
	case *ast.SelectorExpr:
		name = x.Sel.Name
	case *ast.Ident:
		name = x.Name
	default:
		return ""
	}
	switch name {
	case "Extra", "extra":
		return "extra"
	case "Credentials", "credentials", "creds":
		return "credentials"
	}
	return ""
}

func (s *admissionScan) collectBodies(f *ast.File, file string) {
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		name := fd.Name.Name
		if s.calls[name] == nil {
			s.calls[name] = map[string]bool{}
		}
		if admissionSchedFiles[file] {
			s.enterable[name] = true
		}
		if fd.Recv != nil && len(fd.Recv.List) > 0 && admissionIsAccount(fd.Recv.List[0].Type) {
			s.enterable[name] = true
		}
		if fd.Recv == nil && fd.Type.Params != nil {
			for _, prm := range fd.Type.Params.List {
				if admissionIsAccount(prm.Type) {
					s.enterable[name] = true
				}
			}
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.IndexExpr:
				if kind := admissionFieldKind(x.X); kind != "" {
					pos := s.fset.Position(x.Pos()).String()
					if k, ok := s.strOf(x.Index); ok {
						s.reads = append(s.reads, admissionRead{kind, k, name, pos})
					} else {
						s.dynamic = append(s.dynamic, admissionRead{kind, "", name, pos})
					}
				}
			case *ast.CallExpr:
				callee := ""
				switch fn := x.Fun.(type) {
				case *ast.SelectorExpr:
					callee = fn.Sel.Name
				case *ast.Ident:
					callee = fn.Name
				}
				if callee == "" {
					return true
				}
				s.calls[name][callee] = true
				pos := s.fset.Position(x.Pos()).String()
				if kind, ok := admissionKeyMethods[callee]; ok && len(x.Args) > 0 {
					if _, isSel := x.Fun.(*ast.SelectorExpr); isSel {
						if k, ok := s.strOf(x.Args[0]); ok {
							s.reads = append(s.reads, admissionRead{kind, k, name, pos})
						} else {
							s.dynamic = append(s.dynamic, admissionRead{kind, "", name, pos})
						}
					}
				}
				if kind, ok := s.helpers[callee]; ok && len(x.Args) > 1 {
					for _, arg := range x.Args[1:] {
						if k, ok := s.strOf(arg); ok {
							s.reads = append(s.reads, admissionRead{kind, k, name, pos})
						}
					}
				}
				for i, arg := range x.Args {
					kind := admissionFieldKind(arg)
					if kind == "" || i+1 >= len(x.Args) {
						continue
					}
					if k, ok := s.strOf(x.Args[i+1]); ok {
						s.reads = append(s.reads, admissionRead{kind, k, name, pos})
					}
				}
			}
			return true
		})
	}
}

func (s *admissionScan) callersOf(names ...string) []string {
	var out []string
	for fn, cs := range s.calls {
		for _, n := range names {
			if cs[n] {
				out = append(out, fn)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func (s *admissionScan) reach(roots []string, stop map[string]bool) map[string]bool {
	seen := map[string]bool{}
	queue := append([]string(nil), roots...)
	for _, r := range roots {
		seen[r] = true
	}
	for len(queue) > 0 {
		fn := queue[0]
		queue = queue[1:]
		for callee := range s.calls[fn] {
			if seen[callee] || stop[callee] || !s.enterable[callee] {
				continue
			}
			seen[callee] = true
			queue = append(queue, callee)
		}
	}
	return seen
}
