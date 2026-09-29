package handler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// profitVetoLoopResult 记录一次模拟选号循环的终止方式与步数。
type profitVetoLoopResult struct {
	outcome     string // "forwarded" | "exhausted" | "canceled" | "budget_exceeded"
	forwardedID int64
	iterations  int
}

// runProfitVetoLoop 模拟 handler 的「选号 → 利润终检 → 重选」循环，只保留与
// 活锁相关的状态机（FailoverState + 排除列表 + HandleSelectionExhausted），
// 不涉及真实调度器与上游转发。
//
// pool 按顺序给出候选账号；vetoed 中的账号每次终检都被利润门否决——这对应
// 「候选池快照与 per-account 快照短暂不一致」时门的确定性判定。
// maxIterations 是测试自身的预算：循环超过它即视为活锁。
func runProfitVetoLoop(t *testing.T, fs *FailoverState, pool []int64, vetoed map[int64]bool, maxIterations int) profitVetoLoopResult {
	t.Helper()
	res := profitVetoLoopResult{}
	for res.iterations = 1; res.iterations <= maxIterations; res.iterations++ {
		// 选号：返回第一个不在排除列表中的账号。
		var picked int64
		for _, id := range pool {
			if _, excluded := fs.FailedAccountIDs[id]; !excluded {
				picked = id
				break
			}
		}
		if picked == 0 {
			// 选号耗尽：与 handler 一致，只区分客户端断开与耗尽。
			if fs.HandleSelectionExhausted(context.Background()) == FailoverCanceled {
				res.outcome = "canceled"
			} else {
				res.outcome = "exhausted"
			}
			return res
		}
		if vetoed[picked] {
			if fs.RecordProfitVeto(picked) == FailoverExhausted {
				res.outcome = "exhausted"
				return res
			}
			continue
		}
		res.outcome = "forwarded"
		res.forwardedID = picked
		return res
	}
	res.outcome = "budget_exceeded"
	return res
}

// TestProfitVetoAfter503DoesNotLivelock 钉死 #4925 的活锁不再出现：一次真实 503 之后，
// 调度器持续返回会被利润门否决的账号时，循环必须有限步、且不等待地终止
// （选号耗尽不再清空排除列表回头重试，2026-09-29 定）。
func TestProfitVetoAfter503DoesNotLivelock(t *testing.T) {
	fs := NewFailoverState(10, false)
	fs.LastFailoverErr = newTestFailoverErr(503, false, false)
	fs.SwitchCount = 1
	fs.FailedAccountIDs[1] = struct{}{}

	start := time.Now()
	res := runProfitVetoLoop(t, fs, []int64{2, 1}, map[int64]bool{2: true}, 50)
	elapsed := time.Since(start)

	require.Equal(t, "exhausted", res.outcome, "整池被利润门否决 / 已失败时必须有限步终止")
	require.Equal(t, 2, res.iterations, "否决账号 2 后选号即耗尽，不得回头重选已 503 的账号 1")
	require.Less(t, elapsed, time.Second, "不得退避等待")
}

// TestProfitVetoAttemptsCapped 钉死没有 503 参与时，大分组整池越线也会在
// 常数步内终止，而不是把整池逐个选一遍。
func TestProfitVetoAttemptsCapped(t *testing.T) {
	fs := NewFailoverState(10, false)
	pool := make([]int64, 0, 64)
	vetoed := make(map[int64]bool, 64)
	for id := int64(1); id <= 64; id++ {
		pool = append(pool, id)
		vetoed[id] = true
	}

	res := runProfitVetoLoop(t, fs, pool, vetoed, 200)

	require.Equal(t, "exhausted", res.outcome)
	require.Equal(t, maxProfitVetoAttempts, fs.ProfitVetoCount())
	require.Equal(t, maxProfitVetoAttempts, res.iterations, "达到上限即终止，不应继续遍历候选池")
}

// TestRecordProfitVetoExcludesAccount 钉死 RecordProfitVeto 仍然把账号加入
// 调度排除列表（选号入参用的就是 FailedAccountIDs）。
func TestRecordProfitVetoExcludesAccount(t *testing.T) {
	fs := NewFailoverState(10, false)
	require.Equal(t, FailoverContinue, fs.RecordProfitVeto(42))
	require.Contains(t, fs.FailedAccountIDs, int64(42))
	require.Equal(t, 1, fs.ProfitVetoCount())
}
