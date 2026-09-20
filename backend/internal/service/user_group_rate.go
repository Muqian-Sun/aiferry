package service

import "context"

// UserGroupRateEntry 分组下用户专属 RPM 条目（倍率已归用户：users.rate_multiplier）。
// RPMOverride 为指针以支持"未设置"语义（NULL）。
type UserGroupRateEntry struct {
	UserID      int64  `json:"user_id"`
	UserName    string `json:"user_name"`
	UserEmail   string `json:"user_email"`
	UserNotes   string `json:"user_notes"`
	UserStatus  string `json:"user_status"`
	RPMOverride *int   `json:"rpm_override,omitempty"`
}

// GroupRPMOverrideInput 批量设置分组 RPM override 的输入条目。
// RPMOverride 为 *int 以支持清除（nil）语义。
type GroupRPMOverrideInput struct {
	UserID      int64 `json:"user_id"`
	RPMOverride *int  `json:"rpm_override"`
}

// UserGroupRateRepository 用户专属分组 RPM 仓储接口（表名沿用 user_group_rate_multipliers，
// 倍率列自 PR-4a 起不再读写；分组与表在 PR-7 一起删）。
type UserGroupRateRepository interface {
	// GetRPMOverrideByUserAndGroup 获取用户在特定分组的 rpm_override（NULL 返回 nil）
	GetRPMOverrideByUserAndGroup(ctx context.Context, userID, groupID int64) (*int, error)

	// GetByGroupID 获取指定分组下所有用户的专属 RPM 配置（rpm_override 非 NULL 的行）
	GetByGroupID(ctx context.Context, groupID int64) ([]UserGroupRateEntry, error)

	// SyncGroupRPMOverrides 批量同步分组的用户专属 RPM（替换整组 rpm_override 部分）。
	// 条目中 RPMOverride 为 nil 时清空对应行的 rpm_override；非 nil 时 upsert。
	SyncGroupRPMOverrides(ctx context.Context, groupID int64, entries []GroupRPMOverrideInput) error

	// ClearGroupRPMOverrides 清空指定分组的所有 rpm_override（整组 rpm 部分归 NULL）
	ClearGroupRPMOverrides(ctx context.Context, groupID int64) error

	// DeleteByGroupID 删除指定分组的所有用户专属条目（分组删除时调用）
	DeleteByGroupID(ctx context.Context, groupID int64) error

	// DeleteByUserID 删除指定用户的所有专属条目（用户删除时调用）
	DeleteByUserID(ctx context.Context, userID int64) error
}
