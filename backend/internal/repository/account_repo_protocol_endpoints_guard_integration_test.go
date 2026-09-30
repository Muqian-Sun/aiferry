//go:build integration

package repository

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 「第三方 key 必须有协议地址」守在仓储层。管理后台编辑账号走 Update，
// 它必须拦住两种写入：把已有映射清空，以及把成品号改成第三方 key 却不配地址。
func TestAccountUpdatePathsRejectThirdPartyKeyWithoutEndpoints(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newAccountRepositoryWithSQL(tx.Client(), tx, nil)

	paths := []struct {
		name   string
		update func(*service.Account) error
	}{
		{"Update", func(a *service.Account) error { return repo.Update(ctx, a) }},
	}
	for _, path := range paths {
		t.Run(path.name+"/clear endpoints", func(t *testing.T) {
			endpoints := map[string]string{service.APIProtocolChatCompletions: "https://api.openai.com"}
			created := mustCreateAccount(t, tx.Client(), &service.Account{
				Name: "guard-clear-" + path.name, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
				Credentials:       map[string]any{"api_key": "sk-guard"},
				ProtocolEndpoints: endpoints,
			})
			loaded, err := repo.GetByID(ctx, created.ID)
			require.NoError(t, err)
			loaded.ProtocolEndpoints = map[string]string{}

			err = path.update(loaded)

			require.Error(t, err)
			require.Equal(t, "INVALID_PROTOCOL_ENDPOINTS", infraerrors.Reason(err))
			persisted, err := repo.GetByID(ctx, created.ID)
			require.NoError(t, err)
			require.Equal(t, endpoints, persisted.ProtocolEndpoints)
		})

		t.Run(path.name+"/retype to key without endpoints", func(t *testing.T) {
			created := mustCreateAccount(t, tx.Client(), &service.Account{
				Name: "guard-retype-" + path.name, Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth,
				Credentials: map[string]any{"refresh_token": "rt-guard"},
			})
			loaded, err := repo.GetByID(ctx, created.ID)
			require.NoError(t, err)
			loaded.Type = service.AccountTypeAPIKey
			loaded.Credentials = map[string]any{"api_key": "sk-guard"}

			err = path.update(loaded)

			require.Error(t, err)
			require.Equal(t, "INVALID_PROTOCOL_ENDPOINTS", infraerrors.Reason(err))
			persisted, err := repo.GetByID(ctx, created.ID)
			require.NoError(t, err)
			require.Equal(t, service.AccountTypeOAuth, persisted.Type)
		})
	}
}
