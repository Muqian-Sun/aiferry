package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestBuildOpsErrorLogsWhere_QueryUsesQualifiedColumns(t *testing.T) {
	filter := &service.OpsErrorLogFilter{
		Query: "ACCESS_DENIED",
	}

	where, args := buildOpsErrorLogsWhere(filter)
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if !strings.Contains(where, "e.request_id ILIKE $") {
		t.Fatalf("where should include qualified request_id condition: %s", where)
	}
	if !strings.Contains(where, "e.client_request_id ILIKE $") {
		t.Fatalf("where should include qualified client_request_id condition: %s", where)
	}
	if !strings.Contains(where, "e.error_message ILIKE $") {
		t.Fatalf("where should include qualified error_message condition: %s", where)
	}
}

func TestBuildOpsErrorLogsWhere_UserQueryUsesExistsSubquery(t *testing.T) {
	filter := &service.OpsErrorLogFilter{
		UserQuery: "admin@",
	}

	where, args := buildOpsErrorLogsWhere(filter)
	if where == "" {
		t.Fatalf("where should not be empty")
	}
	if len(args) != 1 {
		t.Fatalf("args len = %d, want 1", len(args))
	}
	if !strings.Contains(where, "EXISTS (SELECT 1 FROM users u WHERE u.id = e.user_id AND u.email ILIKE $") {
		t.Fatalf("where should include EXISTS user email condition: %s", where)
	}
}

// 运维看板按模型、渠道筛选（2026-10-04）：用量按实际计费的 model，错误按「请求的模型优先」，渠道都按 account_id。
func TestBuildOpsDashboardWhere_ModelAndAccount(t *testing.T) {
	filter := &service.OpsDashboardFilter{Model: " gpt-5.5 ", AccountID: 7}
	start, end := time.Unix(0, 0), time.Unix(60, 0)

	join, usageWhere, usageArgs, next := buildUsageWhere(filter, start, end, 1)
	if join != "" {
		t.Fatalf("model / account scope needs no join: %q", join)
	}
	if !strings.Contains(usageWhere, "ul.model = $3") || !strings.Contains(usageWhere, "ul.account_id = $4") {
		t.Fatalf("usage where missing model / account: %s", usageWhere)
	}
	if len(usageArgs) != 4 || usageArgs[2] != "gpt-5.5" || usageArgs[3] != int64(7) || next != 5 {
		t.Fatalf("usage args = %v next = %d", usageArgs, next)
	}

	errorWhere, errorArgs, _ := buildErrorWhere(filter, start, end, next)
	if !strings.Contains(errorWhere, "COALESCE(requested_model, model, '') = $7") || !strings.Contains(errorWhere, "account_id = $8") {
		t.Fatalf("error where missing model / account: %s", errorWhere)
	}
	if len(errorArgs) != 4 || errorArgs[2] != "gpt-5.5" || errorArgs[3] != int64(7) {
		t.Fatalf("error args = %v", errorArgs)
	}

	_, plain, plainArgs, _ := buildUsageWhere(&service.OpsDashboardFilter{}, start, end, 1)
	if strings.Contains(plain, "model") || strings.Contains(plain, "account_id") || len(plainArgs) != 2 {
		t.Fatalf("unscoped where should only bound time: %s %v", plain, plainArgs)
	}
}
