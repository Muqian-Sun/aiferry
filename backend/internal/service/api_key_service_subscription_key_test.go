//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// 订阅 key 是订阅的访问凭证：用户不能删、不能改分组；改名允许。

func TestDelete_SubscriptionKeyProtected(t *testing.T) {
	subID := int64(10)
	repo := &apiKeyRepoStub{apiKey: &APIKey{ID: 42, UserID: 7, Key: "k", SubscriptionID: &subID}}
	cache := &apiKeyCacheStub{}
	svc := &APIKeyService{apiKeyRepo: repo, cache: cache}

	err := svc.Delete(context.Background(), 42, 7)
	require.ErrorIs(t, err, ErrSubscriptionKeyProtected)
	require.Empty(t, repo.deletedIDs, "被拒不能删")
	require.Empty(t, cache.deleteAuthKeys, "被拒不动缓存")
}

// 密钥不再有分组：订阅 key 与余额 key 一样只能改名 / 状态等自己的字段。
func TestUpdate_SubscriptionKeyRenameOnlyWritesName(t *testing.T) {
	subID := int64(10)
	svc, repo := newUpdateFieldsAPIKeyService(&APIKey{ID: 1, UserID: 7, Key: "sk-test", Name: "Pro", Status: StatusActive, SubscriptionID: &subID})

	name := "renamed"
	updated, err := svc.Update(context.Background(), 1, 7, UpdateAPIKeyRequest{Name: &name})
	require.NoError(t, err)
	require.Equal(t, "renamed", updated.Name)
	require.Equal(t, []APIKeyUpdateFields{{Name: true}}, repo.updateFields, "改名只写 name 列")
}
