//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// privacyGateMockRepo 记录 SetError 调用：require_privacy_set 只能在本分组内排除账号，
// 不能把共享的账号全局置成 error。
type privacyGateMockRepo struct {
	*groupAwareMockAccountRepo
	setErrorIDs []int64
}

func (m *privacyGateMockRepo) SetError(_ context.Context, id int64, _ string) error {
	m.setErrorIDs = append(m.setErrorIDs, id)
	return nil
}

// 第三方 key 的 IsPrivacySet 恒为 false：绑进 require_privacy_set 分组后，
// Anthropic 网关选号必须只跳过它并选中已确认隐私设置的成品号，而不是把它全局停号——
// 另一个分组可能有意允许这把 key。
func TestAnthropicScheduling_PrivacyGateExcludesWithoutMarkingError(t *testing.T) {
	ctx := context.Background()
	groupID := int64(100)
	accounts := []Account{
		{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Priority: 1, Status: StatusActive, Schedulable: true,
			ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://relay.example.com"},
			AccountGroups:     []AccountGroup{{GroupID: groupID}}},
		{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Priority: 2, Status: StatusActive, Schedulable: true,
			AccountGroups: []AccountGroup{{GroupID: groupID}}},
	}
	repo := &privacyGateMockRepo{groupAwareMockAccountRepo: newGroupAwareMockRepo(accounts)}
	svc := &GatewayService{
		accountRepo: repo,
		groupRepo:   &mockGroupRepoForGateway{groups: map[int64]*Group{groupID: {ID: groupID, Name: "privacy", Platform: PlatformAnthropic, RequirePrivacySet: true}}},
		cache:       &mockGatewayCacheForPlatform{},
		cfg:         testConfig(),
	}

	for name, pick := range map[string]func() (*Account, error){
		"single platform": func() (*Account, error) {
			return svc.selectAccountForModelWithPlatform(ctx, &groupID, "", "", nil, PlatformAnthropic)
		},
		"mixed scheduling": func() (*Account, error) {
			return svc.selectAccountWithMixedScheduling(ctx, &groupID, "", "", nil, PlatformAnthropic)
		},
	} {
		t.Run(name, func(t *testing.T) {
			repo.setErrorIDs = nil
			acc, err := pick()
			require.NoError(t, err)
			require.Equal(t, int64(2), acc.ID, "the subscription account with privacy set must win")
			require.Empty(t, repo.setErrorIDs, "privacy gate must not mutate account status")
		})
	}
}
