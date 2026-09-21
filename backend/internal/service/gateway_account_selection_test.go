//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// --- helpers ---

func testTimePtr(t time.Time) *time.Time { return &t }

func makeAccWithLoad(id int64, priority int, loadRate int, lastUsed *time.Time, accType string) accountWithLoad {
	return accountWithLoad{
		account: &Account{
			ID:          id,
			Priority:    priority,
			LastUsedAt:  lastUsed,
			Type:        accType,
			Schedulable: true,
			Status:      StatusActive,
		},
		loadInfo: &AccountLoadInfo{
			AccountID:          id,
			CurrentConcurrency: 0,
			LoadRate:           loadRate,
		},
	}
}

// --- sortAccountsByPriorityAndLastUsed ---

func TestSortAccountsByPriorityAndLastUsed_ByPriority(t *testing.T) {
	now := time.Now()
	accounts := []*Account{
		{ID: 1, Priority: 5, LastUsedAt: testTimePtr(now)},
		{ID: 2, Priority: 1, LastUsedAt: testTimePtr(now)},
		{ID: 3, Priority: 3, LastUsedAt: testTimePtr(now)},
	}
	sortAccountsByPriorityAndLastUsed(accounts, "")
	require.Equal(t, int64(2), accounts[0].ID, "优先级最低的排第一")
	require.Equal(t, int64(3), accounts[1].ID)
	require.Equal(t, int64(1), accounts[2].ID)
}

func TestSortAccountsByPriorityAndLastUsed_SamePriorityByLastUsed(t *testing.T) {
	now := time.Now()
	accounts := []*Account{
		{ID: 1, Priority: 1, LastUsedAt: testTimePtr(now)},
		{ID: 2, Priority: 1, LastUsedAt: testTimePtr(now.Add(-1 * time.Hour))},
		{ID: 3, Priority: 1, LastUsedAt: nil},
	}
	sortAccountsByPriorityAndLastUsed(accounts, "")
	require.Equal(t, int64(3), accounts[0].ID, "nil LastUsedAt 排最前")
	require.Equal(t, int64(2), accounts[1].ID, "更早使用的排前面")
	require.Equal(t, int64(1), accounts[2].ID)
}

func TestSortAccountsByPriorityAndLastUsed_StableSort(t *testing.T) {
	accounts := []*Account{
		{ID: 1, Priority: 1, LastUsedAt: nil, Type: AccountTypeAPIKey},
		{ID: 2, Priority: 1, LastUsedAt: nil, Type: AccountTypeAPIKey},
		{ID: 3, Priority: 1, LastUsedAt: nil, Type: AccountTypeAPIKey},
	}

	// sortAccountsByPriorityAndLastUsed 内部会在同组(Priority+LastUsedAt)内做随机打散，
	// 因此这里不再断言“稳定排序”。我们只验证：
	// 1) 元素集合不变；2) 多次运行能产生不同的顺序。
	seenFirst := map[int64]bool{}
	for i := 0; i < 100; i++ {
		cpy := make([]*Account, len(accounts))
		copy(cpy, accounts)
		sortAccountsByPriorityAndLastUsed(cpy, "")
		seenFirst[cpy[0].ID] = true

		ids := map[int64]bool{}
		for _, a := range cpy {
			ids[a.ID] = true
		}
		require.True(t, ids[1] && ids[2] && ids[3])
	}
	require.GreaterOrEqual(t, len(seenFirst), 2, "同组账号应能被随机打散")
}

func TestSortAccountsByPriorityAndLastUsed_MixedPriorityAndTime(t *testing.T) {
	now := time.Now()
	accounts := []*Account{
		{ID: 1, Priority: 2, LastUsedAt: nil},
		{ID: 2, Priority: 1, LastUsedAt: testTimePtr(now)},
		{ID: 3, Priority: 1, LastUsedAt: testTimePtr(now.Add(-1 * time.Hour))},
		{ID: 4, Priority: 2, LastUsedAt: testTimePtr(now.Add(-2 * time.Hour))},
	}
	sortAccountsByPriorityAndLastUsed(accounts, "")
	// 优先级1排前：nil < earlier
	require.Equal(t, int64(3), accounts[0].ID, "优先级1 + 更早")
	require.Equal(t, int64(2), accounts[1].ID, "优先级1 + 现在")
	// 优先级2排后：nil < time
	require.Equal(t, int64(1), accounts[2].ID, "优先级2 + nil")
	require.Equal(t, int64(4), accounts[3].ID, "优先级2 + 有时间")
}

// --- filterByMinPriority ---

func TestFilterByMinPriority_Empty(t *testing.T) {
	result := filterByMinPriority(nil)
	require.Nil(t, result)
}

func TestFilterByMinPriority_SelectsMinPriority(t *testing.T) {
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 5, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(2, 1, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(3, 1, 20, nil, AccountTypeAPIKey),
		makeAccWithLoad(4, 2, 10, nil, AccountTypeAPIKey),
	}
	result := filterByMinPriority(accounts)
	require.Len(t, result, 2)
	require.Equal(t, int64(2), result[0].account.ID)
	require.Equal(t, int64(3), result[1].account.ID)
}

// --- filterByMinLoadRate ---

func TestFilterByMinLoadRate_Empty(t *testing.T) {
	result := filterByMinLoadRate(nil)
	require.Nil(t, result)
}

func TestFilterByMinLoadRate_SelectsMinLoadRate(t *testing.T) {
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 1, 30, nil, AccountTypeAPIKey),
		makeAccWithLoad(2, 1, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(3, 1, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(4, 1, 20, nil, AccountTypeAPIKey),
	}
	result := filterByMinLoadRate(accounts)
	require.Len(t, result, 2)
	require.Equal(t, int64(2), result[0].account.ID)
	require.Equal(t, int64(3), result[1].account.ID)
}

// --- selectByLRU ---

func TestSelectByLRU_Empty(t *testing.T) {
	result := selectByLRU(nil)
	require.Nil(t, result)
}

func TestSelectByLRU_Single(t *testing.T) {
	accounts := []accountWithLoad{makeAccWithLoad(1, 1, 10, nil, AccountTypeAPIKey)}
	result := selectByLRU(accounts)
	require.NotNil(t, result)
	require.Equal(t, int64(1), result.account.ID)
}

func TestSelectByLRU_NilLastUsedAtWins(t *testing.T) {
	now := time.Now()
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 1, 10, testTimePtr(now), AccountTypeAPIKey),
		makeAccWithLoad(2, 1, 10, nil, AccountTypeAPIKey),
		makeAccWithLoad(3, 1, 10, testTimePtr(now.Add(-1*time.Hour)), AccountTypeAPIKey),
	}
	result := selectByLRU(accounts)
	require.NotNil(t, result)
	require.Equal(t, int64(2), result.account.ID)
}

func TestSelectByLRU_EarliestTimeWins(t *testing.T) {
	now := time.Now()
	accounts := []accountWithLoad{
		makeAccWithLoad(1, 1, 10, testTimePtr(now), AccountTypeAPIKey),
		makeAccWithLoad(2, 1, 10, testTimePtr(now.Add(-1*time.Hour)), AccountTypeAPIKey),
		makeAccWithLoad(3, 1, 10, testTimePtr(now.Add(-2*time.Hour)), AccountTypeAPIKey),
	}
	result := selectByLRU(accounts)
	require.NotNil(t, result)
	require.Equal(t, int64(3), result.account.ID)
}

// --- 协议匹配第一键 ---

func protocolRankTestAccounts() (direct, converting *Account) {
	direct = &Account{ID: 1, Priority: 5, Type: AccountTypeAPIKey, Platform: PlatformAnthropic,
		ProtocolEndpoints: map[string]string{APIProtocolAnthropic: "https://relay.example.com"}}
	converting = &Account{ID: 2, Priority: 1, Type: AccountTypeAPIKey, Platform: PlatformOpenAI,
		ProtocolEndpoints: map[string]string{APIProtocolResponses: "https://relay.example.com"}}
	return direct, converting
}

func TestSortAccountsByPriorityAndLastUsed_ProtocolMatchBeatsPriority(t *testing.T) {
	direct, converting := protocolRankTestAccounts()
	accounts := []*Account{converting, direct}
	sortAccountsByPriorityAndLastUsed(accounts, APIProtocolAnthropic)
	require.Equal(t, int64(1), accounts[0].ID, "anthropic 直连的排前，虽然优先级数值更大")

	accounts = []*Account{direct, converting}
	sortAccountsByPriorityAndLastUsed(accounts, APIProtocolResponses)
	require.Equal(t, int64(2), accounts[0].ID, "换成 responses 入站，直连的是另一把 key")

	accounts = []*Account{direct, converting}
	sortAccountsByPriorityAndLastUsed(accounts, APIProtocolChatCompletions)
	require.Equal(t, int64(2), accounts[0].ID, "都要转换时按配置的优先级")
}

func TestSortAccountsByPriorityOnly_ProtocolMatchFirst(t *testing.T) {
	direct, converting := protocolRankTestAccounts()
	accounts := []*Account{converting, direct}
	sortAccountsByPriorityOnly(accounts, APIProtocolAnthropic)
	require.Equal(t, int64(1), accounts[0].ID)
}

func TestCandidatePrecedes(t *testing.T) {
	direct, converting := protocolRankTestAccounts()
	require.True(t, candidatePrecedes(direct, converting, APIProtocolAnthropic), "直连先于优先级")
	require.False(t, candidatePrecedes(converting, direct, APIProtocolAnthropic))
	require.True(t, candidatePrecedes(converting, direct, APIProtocolChatCompletions), "都转换时优先级小的先")
	now := time.Now()
	older := &Account{ID: 3, Priority: 1, LastUsedAt: testTimePtr(now.Add(-time.Hour))}
	newer := &Account{ID: 4, Priority: 1, LastUsedAt: testTimePtr(now)}
	never := &Account{ID: 5, Priority: 1}
	require.True(t, candidatePrecedes(older, newer, ""))
	require.True(t, candidatePrecedes(never, older, ""), "从未用过的先")
	require.False(t, candidatePrecedes(never, &Account{ID: 6, Priority: 1}, ""), "都没用过不换")
}

func TestFilterByProtocolMatch(t *testing.T) {
	direct, converting := protocolRankTestAccounts()
	available := []accountWithLoad{
		{account: converting, loadInfo: &AccountLoadInfo{AccountID: 2}},
		{account: direct, loadInfo: &AccountLoadInfo{AccountID: 1}},
	}
	matched := filterByProtocolMatch(available, APIProtocolAnthropic)
	require.Len(t, matched, 1)
	require.Equal(t, int64(1), matched[0].account.ID)
	require.Len(t, filterByProtocolMatch(available, APIProtocolChatCompletions), 2, "没有直连的就原样返回")
	require.Len(t, filterByProtocolMatch(available, ""), 2, "扩展端点没有直连概念")
}
