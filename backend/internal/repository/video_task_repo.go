package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// videoTaskRepository 读写 video_tasks（视频生成任务）。
type videoTaskRepository struct {
	db *sql.DB
}

// NewVideoTaskRepository 创建视频任务仓储。
func NewVideoTaskRepository(db *sql.DB) service.VideoTaskRepository {
	return &videoTaskRepository{db: db}
}

const videoTaskColumns = `id, user_id, api_key_id, account_id, catalog_entry_id, model, upstream_model, upstream_kind,
	upstream_task_id, inbound_shape, request, state, progress, video_url, error_code, error_message, duration_seconds,
	resolution, billed_at, created_at, updated_at`

// videoTaskTerminalStates 终态，UpdateVideoTaskResult 不再改写。
const videoTaskTerminalStatesSQL = `('succeeded', 'failed', 'canceled')`

func (r *videoTaskRepository) CreateVideoTask(ctx context.Context, task *service.VideoTask) error {
	request, err := service.MarshalVideoTaskRequest(task.Request)
	if err != nil {
		return fmt.Errorf("marshal video task request: %w", err)
	}
	now := time.Now()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = task.CreatedAt
	}
	if task.State == "" {
		task.State = service.VideoTaskQueued
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO video_tasks (`+videoTaskColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
		task.ID, task.UserID, task.APIKeyID, task.AccountID, task.CatalogEntryID, task.Model, task.UpstreamModel,
		string(task.UpstreamKind), task.UpstreamTaskID, string(task.InboundShape), string(request), string(task.State),
		nullableProgress(task.Progress), task.VideoURL, task.ErrorCode, task.ErrorMessage, task.DurationSeconds,
		task.Resolution, task.BilledAt, task.CreatedAt, task.UpdatedAt)
	return err
}

func (r *videoTaskRepository) GetVideoTask(ctx context.Context, id string, apiKeyID int64) (*service.VideoTask, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+videoTaskColumns+` FROM video_tasks WHERE id = $1 AND api_key_id = $2`, id, apiKeyID)
	var task service.VideoTask
	var upstreamKind, inboundShape, state string
	var request []byte
	var progress sql.NullInt64
	var billedAt sql.NullTime
	err := row.Scan(&task.ID, &task.UserID, &task.APIKeyID, &task.AccountID, &task.CatalogEntryID, &task.Model, &task.UpstreamModel,
		&upstreamKind, &task.UpstreamTaskID, &inboundShape, &request, &state, &progress, &task.VideoURL, &task.ErrorCode,
		&task.ErrorMessage, &task.DurationSeconds, &task.Resolution, &billedAt, &task.CreatedAt, &task.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrVideoTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	task.UpstreamKind = service.VideoUpstreamKind(upstreamKind)
	task.InboundShape = service.VideoInboundShape(inboundShape)
	task.State = service.VideoTaskState(state)
	if progress.Valid {
		value := int(progress.Int64)
		task.Progress = &value
	}
	if billedAt.Valid {
		at := billedAt.Time
		task.BilledAt = &at
	}
	if task.Request, err = service.UnmarshalVideoTaskRequest(request); err != nil {
		return nil, fmt.Errorf("decode request of video task %s: %w", task.ID, err)
	}
	return &task, nil
}

func (r *videoTaskRepository) UpdateVideoTaskResult(ctx context.Context, task *service.VideoTask) error {
	_, err := r.db.ExecContext(ctx, `UPDATE video_tasks
		SET state = $2, progress = $3, video_url = $4, error_code = $5, error_message = $6,
			duration_seconds = $7, resolution = $8, updated_at = NOW()
		WHERE id = $1 AND state NOT IN `+videoTaskTerminalStatesSQL,
		task.ID, string(task.State), nullableProgress(task.Progress), task.VideoURL, task.ErrorCode, task.ErrorMessage,
		task.DurationSeconds, task.Resolution)
	return err
}

func (r *videoTaskRepository) ClaimVideoTaskBilling(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE video_tasks SET billed_at = NOW() WHERE id = $1 AND billed_at IS NULL`, id)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (r *videoTaskRepository) ReleaseVideoTaskBilling(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE video_tasks SET billed_at = NULL WHERE id = $1`, id)
	return err
}

func (r *videoTaskRepository) DeleteVideoTasksBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM video_tasks WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func nullableProgress(progress *int) any {
	if progress == nil {
		return nil
	}
	return *progress
}
