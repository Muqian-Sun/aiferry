//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type staticErrorPassthroughRuleRepo struct {
	rules []*model.ErrorPassthroughRule
}

func (r staticErrorPassthroughRuleRepo) List(context.Context) ([]*model.ErrorPassthroughRule, error) {
	return r.rules, nil
}

func (staticErrorPassthroughRuleRepo) GetByID(context.Context, int64) (*model.ErrorPassthroughRule, error) {
	return nil, nil
}

func (staticErrorPassthroughRuleRepo) Create(context.Context, *model.ErrorPassthroughRule) (*model.ErrorPassthroughRule, error) {
	return nil, nil
}

func (staticErrorPassthroughRuleRepo) Update(context.Context, *model.ErrorPassthroughRule) (*model.ErrorPassthroughRule, error) {
	return nil, nil
}

func (staticErrorPassthroughRuleRepo) Delete(context.Context, int64) error {
	return nil
}

type fixedStatusHTTPUpstream struct {
	status int
	body   string
}

func (u fixedStatusHTTPUpstream) Do(*http.Request, string, int64, int) (*http.Response, error) {
	return &http.Response{
		StatusCode: u.status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(u.body)),
	}, nil
}

func (u fixedStatusHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

// Messages 换号耗尽时匹配错误透传规则：第三方 key 按请求所在网关平台匹配，不看平台标签。
func TestGatewayHandlerMessages_FailoverExhaustedPassthroughRuleUsesGatewayPlatformForKey(t *testing.T) {
	const keyword = "passthrough-platform-probe"
	for _, tt := range []struct {
		name          string
		rulePlatform  string
		wantRuleMatch bool
	}{
		{name: "rule on gateway platform matches", rulePlatform: service.PlatformAnthropic, wantRuleMatch: true},
		{name: "rule on key label does not match", rulePlatform: service.PlatformOpenAI, wantRuleMatch: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			key := keyRouteAccount(1201, service.PlatformOpenAI,
				map[string]string{service.APIProtocolAnthropic: "https://relay.example.com"}, "claude-sonnet-4-5")
			h, cleanup := newTestGatewayHandler(t, []*service.Account{key})
			t.Cleanup(cleanup)

			schedulerCache := &fakeSchedulerCache{accounts: []*service.Account{key}}
			h.gatewayService = service.NewGatewayService(
				nil, nil, nil, nil, nil, nil,
				&config.Config{RunMode: config.RunModeSimple},
				service.NewSchedulerSnapshotService(schedulerCache, nil, nil, nil),
				nil, nil, nil, nil, nil,
				fixedStatusHTTPUpstream{status: http.StatusInternalServerError, body: `{"error":{"message":"` + keyword + `"}}`},
				nil, nil, nil, nil, nil, nil, nil, nil, nil,
			)
			h.maxAccountSwitches = 0
			code := http.StatusTeapot
			h.errorPassthroughService = service.NewErrorPassthroughService(staticErrorPassthroughRuleRepo{rules: []*model.ErrorPassthroughRule{{
				ID:              1,
				Name:            "platform-probe",
				Enabled:         true,
				Priority:        1,
				Keywords:        []string{keyword},
				MatchMode:       model.MatchModeAny,
				Platforms:       []string{tt.rulePlatform},
				ResponseCode:    &code,
				PassthroughBody: true,
			}}}, nil)

			body := []byte(`{"model":"claude-sonnet-4-5","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`)
			c, rec := newKeyRouteContext(t, http.MethodPost, "/v1/messages", body, service.APIProtocolAnthropic, "")

			h.Messages(c)

			if tt.wantRuleMatch {
				require.Equal(t, http.StatusTeapot, rec.Code, rec.Body.String())
			} else {
				require.NotEqual(t, http.StatusTeapot, rec.Code, rec.Body.String())
				require.NotZero(t, rec.Code)
			}
		})
	}
}
