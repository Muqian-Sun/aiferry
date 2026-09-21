package service

const (
	SchedulerOutboxEventAccountChanged       = "account_changed"
	SchedulerOutboxEventAccountGroupsChanged = "account_groups_changed"
	SchedulerOutboxEventAccountBulkChanged   = "account_bulk_changed"
	SchedulerOutboxEventAccountLastUsed      = "account_last_used"
	SchedulerOutboxEventGroupChanged         = "group_changed"
	SchedulerOutboxEventFullRebuild          = "full_rebuild"
	// SchedulerOutboxEventCatalogBindingsChanged 目录条目的资源绑定变了（含条目删除），
	// payload {"entry_ids": [...]}，快照据此重建或退役目录桶。
	SchedulerOutboxEventCatalogBindingsChanged = "catalog_bindings_changed"
)
