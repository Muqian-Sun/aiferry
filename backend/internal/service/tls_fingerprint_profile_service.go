package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

// TLSFingerprintProfileRepository 定义 TLS 指纹模板的数据访问接口
type TLSFingerprintProfileRepository interface {
	List(ctx context.Context) ([]*model.TLSFingerprintProfile, error)
	GetByID(ctx context.Context, id int64) (*model.TLSFingerprintProfile, error)
	Create(ctx context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error)
	Update(ctx context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error)
	Delete(ctx context.Context, id int64) error
}

// TLSFingerprintProfileService TLS 指纹模板管理服务。
//
// 模板表只剩管理端增删改查（界面已藏，接口保留）：出站一律用内置模板（ResolveTLSProfile），
// 渠道不再能绑模板或随机选模板（2026-09-28 P5），所以不再在进程里缓存模板、也不跨实例同步。
type TLSFingerprintProfileService struct {
	repo TLSFingerprintProfileRepository
}

// NewTLSFingerprintProfileService 创建 TLS 指纹模板服务
func NewTLSFingerprintProfileService(repo TLSFingerprintProfileRepository) *TLSFingerprintProfileService {
	return &TLSFingerprintProfileService{repo: repo}
}

// --- CRUD ---

// List 获取所有模板
func (s *TLSFingerprintProfileService) List(ctx context.Context) ([]*model.TLSFingerprintProfile, error) {
	return s.repo.List(ctx)
}

// GetByID 根据 ID 获取模板
func (s *TLSFingerprintProfileService) GetByID(ctx context.Context, id int64) (*model.TLSFingerprintProfile, error) {
	return s.repo.GetByID(ctx, id)
}

// Create 创建模板
func (s *TLSFingerprintProfileService) Create(ctx context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, profile)
}

// Update 更新模板
func (s *TLSFingerprintProfileService) Update(ctx context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, profile)
}

// Delete 删除模板
func (s *TLSFingerprintProfileService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// --- 出站 TLS 指纹 ---

// ResolveTLSProfile 返回账号出站用的 TLS 指纹：要模拟的（Account.IsTLSFingerprintEnabled，现为 Anthropic 成品号）
// 一律用内置模板——空 Profile，dialer 用代码内置的 Node.js 默认值；其余返回 nil（不模拟）。
// 不读 s 的任何字段：服务为 nil 时（部分测试 / 未注入）也能调用。
func (s *TLSFingerprintProfileService) ResolveTLSProfile(account *Account) *tlsfingerprint.Profile {
	if account == nil || !account.IsTLSFingerprintEnabled() {
		return nil
	}
	return &tlsfingerprint.Profile{Name: builtinTLSFingerprintProfileName}
}
