//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestVideoTaskRepositoryRoundTripClaimAndPurge(t *testing.T) {
	ctx := context.Background()
	repo := NewVideoTaskRepository(integrationDB)
	prefix := fmt.Sprintf("vtask_it%d_", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM video_tasks WHERE id LIKE $1`, prefix+"%")
	})
	seed := int64(9007199254740993)
	created := time.Now().UTC().Truncate(time.Millisecond)
	task := &service.VideoTask{
		ID: prefix + "a", UserID: 10, APIKeyID: 20, AccountID: 3, CatalogEntryID: 7, Model: "sora-2", UpstreamModel: "sora-2-relay",
		UpstreamKind: service.VideoUpstreamNewAPIRelay, UpstreamTaskID: "na-1", InboundShape: service.VideoShapeOpenAI,
		Request: service.VideoTaskRequest{Model: "sora-2", Prompt: "waves", DurationSeconds: 8, Resolution: service.VideoResolution720P,
			AspectRatio: "16:9", Seed: &seed, Images: []service.VideoInputImage{{URL: "https://example.com/a.png", Role: service.VideoInputImageFirstFrame}},
			Extra: map[string]any{"style": "anime"}},
		State: service.VideoTaskQueued, CreatedAt: created, UpdatedAt: created,
	}
	require.NoError(t, repo.CreateVideoTask(ctx, task))

	got, err := repo.GetVideoTask(ctx, task.ID, 20)
	require.NoError(t, err)
	require.Equal(t, task.Request.Prompt, got.Request.Prompt)
	require.Equal(t, seed, *got.Request.Seed, "大整数不丢精度")
	require.Equal(t, task.Request.Images, got.Request.Images)
	require.Equal(t, "anime", got.Request.Extra["style"])
	require.Equal(t, service.VideoUpstreamNewAPIRelay, got.UpstreamKind)
	require.Equal(t, service.VideoShapeOpenAI, got.InboundShape)
	require.Nil(t, got.Progress)
	require.Nil(t, got.BilledAt)
	require.True(t, got.CreatedAt.Equal(created))

	_, err = repo.GetVideoTask(ctx, task.ID, 21)
	require.ErrorIs(t, err, service.ErrVideoTaskNotFound, "别的 key 取不到")
	_, err = repo.GetVideoTask(ctx, prefix+"missing", 20)
	require.ErrorIs(t, err, service.ErrVideoTaskNotFound)

	progress := 40
	got.State, got.Progress = service.VideoTaskRunning, &progress
	require.NoError(t, repo.UpdateVideoTaskResult(ctx, got))
	got.State, got.VideoURL, got.DurationSeconds, got.Resolution = service.VideoTaskSucceeded, "https://cdn.example.com/a.mp4", 7.5, service.VideoResolution1080P
	require.NoError(t, repo.UpdateVideoTaskResult(ctx, got))
	got.State, got.ErrorMessage = service.VideoTaskFailed, "late failure"
	require.NoError(t, repo.UpdateVideoTaskResult(ctx, got))
	stored, err := repo.GetVideoTask(ctx, task.ID, 20)
	require.NoError(t, err)
	require.Equal(t, service.VideoTaskSucceeded, stored.State, "终态不再被改写")
	require.Equal(t, 40, *stored.Progress)
	require.Equal(t, "https://cdn.example.com/a.mp4", stored.VideoURL)
	require.InDelta(t, 7.5, stored.DurationSeconds, 1e-9)
	require.Equal(t, service.VideoResolution1080P, stored.Resolution)
	require.Empty(t, stored.ErrorMessage)

	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claimed, err := repo.ClaimVideoTaskBilling(ctx, task.ID)
			require.NoError(t, err)
			if claimed {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), wins.Load(), "并发抢占只有一个成功")
	stored, err = repo.GetVideoTask(ctx, task.ID, 20)
	require.NoError(t, err)
	require.NotNil(t, stored.BilledAt)

	require.NoError(t, repo.ReleaseVideoTaskBilling(ctx, task.ID))
	claimed, err := repo.ClaimVideoTaskBilling(ctx, task.ID)
	require.NoError(t, err)
	require.True(t, claimed, "释放后可以重新抢占")

	old := *task
	old.ID, old.CreatedAt, old.UpdatedAt = prefix+"old", created.Add(-8*24*time.Hour), created.Add(-8*24*time.Hour)
	require.NoError(t, repo.CreateVideoTask(ctx, &old))
	_, err = repo.DeleteVideoTasksBefore(ctx, time.Now().Add(-service.VideoTaskRetention))
	require.NoError(t, err)
	_, err = repo.GetVideoTask(ctx, old.ID, 20)
	require.ErrorIs(t, err, service.ErrVideoTaskNotFound, "过保留期的删掉")
	_, err = repo.GetVideoTask(ctx, task.ID, 20)
	require.NoError(t, err, "保留期内的留着")
}
