package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// schedulingTrafficRepository 给调度总览读时间窗内按模型 × 渠道的请求数。读渠道状态的分钟汇总表，
// 成功 / 失败的口径与渠道状态页一致。
type schedulingTrafficRepository struct {
	db *sql.DB
}

// NewSchedulingTrafficRepository 创建调度总览的请求汇总读取器。
func NewSchedulingTrafficRepository(db *sql.DB) service.SchedulingTrafficSource {
	return &schedulingTrafficRepository{db: db}
}

func (r *schedulingTrafficRepository) SchedulingTraffic(ctx context.Context, since time.Time) ([]service.SchedulingTrafficRow, *time.Time, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT model, account_id, SUM(success_requests), SUM(error_requests)
		FROM channel_monitor_v2_metrics_1m
		WHERE bucket_start >= date_trunc('minute', $1::timestamptz)
		GROUP BY model, account_id`, since)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.SchedulingTrafficRow
	for rows.Next() {
		var row service.SchedulingTrafficRow
		if err := rows.Scan(&row.Model, &row.AccountID, &row.Success, &row.Errors); err != nil {
			return nil, nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var dataThrough sql.NullTime
	err = r.db.QueryRowContext(ctx, `SELECT data_through FROM channel_monitor_v2_watermarks WHERE id = 1`).Scan(&dataThrough)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, err
	}
	if !dataThrough.Valid {
		return out, nil, nil
	}
	through := dataThrough.Time.UTC()
	return out, &through, nil
}
