package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// schedulingTraceRepository 读写 scheduling_traces（请求级调度轨迹）。
type schedulingTraceRepository struct {
	db *sql.DB
}

// NewSchedulingTraceRepository 创建调度轨迹仓储。
func NewSchedulingTraceRepository(db *sql.DB) service.SchedulingTraceRepository {
	return &schedulingTraceRepository{db: db}
}

const schedulingTraceColumns = `created_at, request_id, user_id, api_key_id, model, catalog_entry_id, inbound_protocol,
	outcome, status_code, final_account_id, switch_count, sticky_hit, duration_ms, picks, attempts`

const schedulingTraceColumnCount = 15

func (r *schedulingTraceRepository) InsertSchedulingTraces(ctx context.Context, traces []service.SchedulingTrace) error {
	if len(traces) == 0 {
		return nil
	}
	rows := make([]string, 0, len(traces))
	args := make([]any, 0, len(traces)*schedulingTraceColumnCount)
	for i, trace := range traces {
		picks, err := json.Marshal(trace.Picks)
		if err != nil {
			return fmt.Errorf("marshal picks: %w", err)
		}
		attempts, err := json.Marshal(trace.Attempts)
		if err != nil {
			return fmt.Errorf("marshal attempts: %w", err)
		}
		placeholders := make([]string, schedulingTraceColumnCount)
		for j := range placeholders {
			placeholders[j] = fmt.Sprintf("$%d", i*schedulingTraceColumnCount+j+1)
		}
		rows = append(rows, "("+strings.Join(placeholders, ",")+")")
		args = append(args, trace.CreatedAt, trace.RequestID, trace.UserID, trace.APIKeyID, trace.Model, trace.CatalogEntryID,
			trace.InboundProtocol, trace.Outcome, trace.StatusCode, trace.FinalAccountID, trace.SwitchCount, trace.StickyHit,
			trace.DurationMs, string(picks), string(attempts))
	}
	_, err := r.db.ExecContext(ctx, "INSERT INTO scheduling_traces ("+schedulingTraceColumns+") VALUES "+strings.Join(rows, ","), args...)
	return err
}

func (r *schedulingTraceRepository) DeleteSchedulingTracesBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM scheduling_traces WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *schedulingTraceRepository) ListSchedulingTraces(ctx context.Context, filter service.SchedulingTraceFilter) ([]service.SchedulingTrace, int64, error) {
	where := []string{"created_at >= $1"}
	args := []any{filter.Since}
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	if filter.CatalogEntryID > 0 {
		add("catalog_entry_id = $%d", filter.CatalogEntryID)
	}
	if filter.InboundProtocol != "" {
		add("inbound_protocol = $%d", filter.InboundProtocol)
	}
	if filter.AccountID > 0 {
		add("final_account_id = $%d", filter.AccountID)
	}
	switch filter.Outcome {
	case "":
	case "switched":
		where = append(where, "switch_count > 0")
	default:
		add("outcome = $%d", filter.Outcome)
	}
	clause := " WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM scheduling_traces"+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf("SELECT id, %s FROM scheduling_traces%s ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d",
		schedulingTraceColumns, clause, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.SchedulingTrace{}
	for rows.Next() {
		var trace service.SchedulingTrace
		var userID, apiKeyID, entryID, accountID sql.NullInt64
		var picks, attempts []byte
		if err := rows.Scan(&trace.ID, &trace.CreatedAt, &trace.RequestID, &userID, &apiKeyID, &trace.Model, &entryID, &trace.InboundProtocol,
			&trace.Outcome, &trace.StatusCode, &accountID, &trace.SwitchCount, &trace.StickyHit, &trace.DurationMs, &picks, &attempts); err != nil {
			return nil, 0, err
		}
		trace.UserID, trace.APIKeyID, trace.CatalogEntryID, trace.FinalAccountID = schedulingTraceInt64(userID), schedulingTraceInt64(apiKeyID), schedulingTraceInt64(entryID), schedulingTraceInt64(accountID)
		if err := json.Unmarshal(picks, &trace.Picks); err != nil {
			return nil, 0, fmt.Errorf("decode picks of trace %d: %w", trace.ID, err)
		}
		if err := json.Unmarshal(attempts, &trace.Attempts); err != nil {
			return nil, 0, fmt.Errorf("decode attempts of trace %d: %w", trace.ID, err)
		}
		out = append(out, trace)
	}
	return out, total, rows.Err()
}

func (r *schedulingTraceRepository) CountSchedulingTraces(ctx context.Context, since time.Time) ([]service.SchedulingTraceCount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(catalog_entry_id, 0), inbound_protocol, COALESCE(final_account_id, 0), outcome, COUNT(*)
		FROM scheduling_traces
		WHERE created_at >= $1
		GROUP BY 1, 2, 3, 4`, since)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.SchedulingTraceCount
	for rows.Next() {
		var count service.SchedulingTraceCount
		if err := rows.Scan(&count.CatalogEntryID, &count.InboundProtocol, &count.AccountID, &count.Outcome, &count.Requests); err != nil {
			return nil, err
		}
		out = append(out, count)
	}
	return out, rows.Err()
}

func schedulingTraceInt64(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	return &v.Int64
}
