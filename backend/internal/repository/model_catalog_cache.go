package repository

import (
	"context"
	"log/slog"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/redis/go-redis/v9"
)

const modelCatalogCachePubSubKey = "model_catalog:cache:update"

type modelCatalogCache struct {
	rdb *redis.Client
}

// NewModelCatalogCache 创建模型目录缓存失效广播器。
func NewModelCatalogCache(rdb *redis.Client) service.ModelCatalogCachePubSub {
	return &modelCatalogCache{rdb: rdb}
}

// NotifyUpdate 通知所有实例丢弃本地目录快照。
func (c *modelCatalogCache) NotifyUpdate(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Publish(ctx, modelCatalogCachePubSubKey, "refresh").Err()
}

// SubscribeUpdates 订阅目录缓存失效通知。
func (c *modelCatalogCache) SubscribeUpdates(ctx context.Context, handler func()) {
	if c == nil || c.rdb == nil {
		return
	}
	go func() {
		pubsub := c.rdb.Subscribe(ctx, modelCatalogCachePubSubKey)
		defer func() {
			if err := pubsub.Close(); err != nil {
				slog.Warn("failed to close model catalog cache subscriber", "error", err)
			}
		}()

		messages := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case message, ok := <-messages:
				if !ok {
					slog.Warn("model catalog cache subscriber stopped", "reason", "channel_closed")
					return
				}
				if message != nil {
					handler()
				}
			}
		}
	}()
}
