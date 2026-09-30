package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// auditLogRepository 审计日志仓储（raw SQL，append-only）。
// 刻意不实现单条删除：审计日志只允许追加与按保留期批量清理。
type auditLogRepository struct {
	db *sql.DB
}

// NewAuditLogRepository 创建审计日志仓储。
func NewAuditLogRepository(db *sql.DB) service.AuditLogRepository {
	return &auditLogRepository{db: db}
}

func auditLogInsertValues(log *service.AuditLog) []any {
	createdAt := log.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	extraJSON := "{}"
	if len(log.Extra) > 0 {
		if encoded, err := json.Marshal(log.Extra); err == nil {
			extraJSON = string(encoded)
		}
	}
	return []any{
		createdAt.UTC(),
		nullInt64Ptr(log.ActorUserID),
		truncateString(log.ActorEmail, 255),
		truncateString(log.ActorRole, 32),
		truncateString(log.AuthMethod, 32),
		truncateString(log.CredentialMasked, 160),
		truncateString(log.Action, 128),
		truncateString(log.Method, 16),
		truncateString(log.Path, 512),
		truncateString(log.RequestID, 64),
		truncateString(log.ClientIP, 64),
		truncateString(log.UserAgent, 512),
		log.RequestBody,
		log.StatusCode,
		log.LatencyMs,
		extraJSON,
	}
}

func (r *auditLogRepository) BatchInsert(ctx context.Context, logs []*service.AuditLog) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("nil audit log repository")
	}
	if len(logs) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	stmt, err := tx.PrepareContext(ctx, pq.CopyIn(
		"audit_logs",
		"created_at", "actor_user_id", "actor_email", "actor_role", "auth_method",
		"credential_masked", "action", "method", "path", "request_id", "client_ip", "user_agent",
		"request_body", "status_code", "latency_ms", "extra",
	))
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}

	var inserted int64
	for _, log := range logs {
		if log == nil {
			continue
		}
		if _, err := stmt.ExecContext(ctx, auditLogInsertValues(log)...); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			return inserted, err
		}
		inserted++
	}

	if _, err := stmt.ExecContext(ctx); err != nil {
		_ = stmt.Close()
		_ = tx.Rollback()
		return inserted, err
	}
	if err := stmt.Close(); err != nil {
		_ = tx.Rollback()
		return inserted, err
	}
	if err := tx.Commit(); err != nil {
		return inserted, err
	}
	return inserted, nil
}

func (r *auditLogRepository) DeleteBefore(ctx context.Context, cutoff time.Time, batchSize int) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("nil audit log repository")
	}
	if batchSize <= 0 {
		batchSize = 5000
	}
	res, err := r.db.ExecContext(ctx, `
WITH batch AS (
  SELECT id FROM audit_logs WHERE created_at < $1 ORDER BY id LIMIT $2
)
DELETE FROM audit_logs WHERE id IN (SELECT id FROM batch)`, cutoff.UTC(), batchSize)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func nullInt64Ptr(v *int64) any {
	if v == nil || *v <= 0 {
		return nil
	}
	return *v
}

func truncateString(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	// 按字节截断可能切断多字节字符，按 rune 处理。
	runes := []rune(s)
	for len(string(runes)) > max && len(runes) > 0 {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}
