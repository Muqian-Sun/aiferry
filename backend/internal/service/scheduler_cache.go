package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// SchedulerModeSingle 平台池：PoolID 恒 0，候选 = 该平台的全部可调度资源（无模型端点用）。
	SchedulerModeSingle = "single"
	// SchedulerModeCatalog 目录桶：PoolID 存的是目录条目 ID，Platform 恒空
	// （条目没有网关族），候选来自 model_catalog_bindings。
	SchedulerModeCatalog = "catalog"
)

var ErrSchedulerBucketWriteFenced = errors.New("scheduler bucket write fenced")

// SchedulerBucketWriteToken fences a snapshot writer to one bucket epoch.
// Tokens must be captured before any database load or queued rebuild work.
type SchedulerBucketWriteToken struct {
	Bucket SchedulerBucket
	Epoch  int64
}

func (t SchedulerBucketWriteToken) ValidFor(bucket SchedulerBucket) bool {
	return t.Epoch > 0 && t.Bucket == bucket
}

// SchedulerBucket 一份候选账号列表的键：目录桶 = (条目 ID, "", catalog)，平台池 = (0, 平台, single)。
type SchedulerBucket struct {
	PoolID   int64
	Platform string
	Mode     string
}

func (b SchedulerBucket) String() string {
	return fmt.Sprintf("%d:%s:%s", b.PoolID, b.Platform, b.Mode)
}

// ParseSchedulerBucket 解析注册表里的桶键；只认平台池与目录桶两种形态，
// 旧的 mixed / forced / 分组桶（PoolID 非 0 的 single）一律拒绝——注册表里的残留成员被忽略，不再重建。
func ParseSchedulerBucket(raw string) (SchedulerBucket, bool) {
	parts := strings.Split(raw, ":")
	if len(parts) != 3 {
		return SchedulerBucket{}, false
	}
	poolID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return SchedulerBucket{}, false
	}
	switch parts[2] {
	case SchedulerModeCatalog:
		if poolID <= 0 || parts[1] != "" {
			return SchedulerBucket{}, false
		}
	case SchedulerModeSingle:
		if poolID != 0 || parts[1] == "" {
			return SchedulerBucket{}, false
		}
	default:
		return SchedulerBucket{}, false
	}
	return SchedulerBucket{
		PoolID:   poolID,
		Platform: parts[1],
		Mode:     parts[2],
	}, true
}

// SchedulerCache 负责调度快照与账号快照的缓存读写。
type SchedulerCache interface {
	// GetSnapshot 读取快照并返回命中与否（ready + active + 数据完整）。
	GetSnapshot(ctx context.Context, bucket SchedulerBucket) ([]*Account, bool, error)
	// CaptureBucketWriteToken captures the current writer epoch.
	CaptureBucketWriteToken(ctx context.Context, bucket SchedulerBucket) (SchedulerBucketWriteToken, error)
	// SetSnapshot 写入快照并切换激活版本。token 必须在 DB load/任务排队前取得。
	SetSnapshot(ctx context.Context, bucket SchedulerBucket, token SchedulerBucketWriteToken, accounts []Account) error
	// GetAccount 获取单账号快照。
	GetAccount(ctx context.Context, accountID int64) (*Account, error)
	// SetAccount 写入单账号快照（包含不可调度状态）。
	SetAccount(ctx context.Context, account *Account) error
	// DeleteAccount 删除单账号快照。
	DeleteAccount(ctx context.Context, accountID int64) error
	// UpdateLastUsed 批量更新账号的最后使用时间。
	UpdateLastUsed(ctx context.Context, updates map[int64]time.Time) error
	// TryLockBucket 尝试获取分桶重建锁。
	TryLockBucket(ctx context.Context, bucket SchedulerBucket, ttl time.Duration) (bool, error)
	// UnlockBucket 释放分桶重建锁。
	UnlockBucket(ctx context.Context, bucket SchedulerBucket) error
	// ListBuckets 返回已注册的分桶集合。
	ListBuckets(ctx context.Context) ([]SchedulerBucket, error)
	// GetOutboxWatermark 读取 outbox 水位。
	GetOutboxWatermark(ctx context.Context) (int64, error)
	// SetOutboxWatermark 保存 outbox 水位。
	SetOutboxWatermark(ctx context.Context, id int64) error
}
