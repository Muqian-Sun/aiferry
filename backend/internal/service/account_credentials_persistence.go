package service

import (
	"context"
	"log/slog"
	"net/http"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type accountCredentialsUpdater interface {
	UpdateCredentials(ctx context.Context, id int64, credentials map[string]any) error
}

func persistAccountCredentials(ctx context.Context, repo AccountRepository, account *Account, credentials map[string]any) error {
	if repo == nil || account == nil {
		return nil
	}

	// 安全不变量:spark 影子账号恒不持凭据(凭据透传母账号)。这是凭据写入的唯一汇聚点
	// (token 刷新 / 订阅补全等全部经此),在此对影子早返 no-op 是
	// defense-in-depth——即便某条上游路径漏判,也不会把凭据落到影子行(外审第6轮 P1)。
	if account.IsCredentialShadow() {
		slog.Warn("skip persisting credentials to spark shadow account",
			"account_id", account.ID, "parent_id", *account.ParentAccountID)
		return nil
	}

	account.Credentials = shallowCopyMap(credentials)
	if updater, ok := any(repo).(accountCredentialsUpdater); ok {
		return updater.UpdateCredentials(ctx, account.ID, account.Credentials)
	}
	return repo.Update(ctx, account)
}

// removedModelMappingCredentialKeys 渠道上的「模型改名」已删（D4，muqian 2026-10-01）：改名在价格页每条承接关系的
// 上游模型名里设置。管理端建号 / 改号 / 批量改号带这两个键直接拒绝，不静默丢掉——调用方以为设上了的改名其实不生效。
// spark 影子号的 model_mapping 是系统维护的模型列表，走影子号自己的更新路径，不经过这里。
var removedModelMappingCredentialKeys = []string{"model_mapping", "model_mapping_rename_only"}

func rejectRemovedModelMappingCredentials(credentials map[string]any) error {
	for _, key := range removedModelMappingCredentialKeys {
		if _, ok := credentials[key]; ok {
			return infraerrors.Newf(http.StatusBadRequest, "ACCOUNT_MODEL_MAPPING_REMOVED",
				"credentials.%s is no longer supported: set the upstream model name per model on the pricing page", key)
		}
	}
	return nil
}

// sparkShadowAllowedCredentialKeys 是 spark 影子账号唯一可写的凭据键集合(仅模型映射)。
// 校验(isAllowed)与 sanitize 共用此单一来源,避免两处独立硬编码列表漂移。
var sparkShadowAllowedCredentialKeys = map[string]struct{}{
	"model_mapping": {},
}

func isAllowedSparkShadowCredentialsUpdate(credentials map[string]any) bool {
	if credentials == nil {
		return true
	}
	for key := range credentials {
		if _, ok := sparkShadowAllowedCredentialKeys[key]; !ok {
			return false
		}
	}
	return true
}

func sanitizeSparkShadowCredentials(credentials map[string]any) map[string]any {
	if len(credentials) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(sparkShadowAllowedCredentialKeys))
	for key := range sparkShadowAllowedCredentialKeys {
		if value, ok := credentials[key]; ok && value != nil {
			out[key] = value
		}
	}
	return out
}
