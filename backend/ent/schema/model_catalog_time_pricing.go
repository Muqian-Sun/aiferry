package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ModelCatalogTimePricing 是目录条目的分时倍率配置，每个条目至多一行。
//
// periods 保存为 JSONB 数组（[{start_time,end_time,multiplier}]）而不是一档一行：
// timezone + weekdays_only + 有序时段是一个整体，service.TimePricing.MultiplierAt
// 对它们做整体判定，拆成行只会在每次读取时再拼回来，而且没有任何查询按单个时段过滤。
type ModelCatalogTimePricing struct {
	ent.Schema
}

func (ModelCatalogTimePricing) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "model_catalog_time_pricing"},
	}
}

func (ModelCatalogTimePricing) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ModelCatalogTimePricing) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("entry_id").
			Unique().
			Comment("指向 model_catalog_entries.id，一个条目至多一份分时配置。"),
		field.String("timezone").
			MaxLen(64).
			Default("").
			Comment("时段判定所用时区；空表示服务器时区。"),
		field.Bool("weekdays_only").
			Default(false).
			Comment("true 表示仅工作日生效，周末恒为 1 倍。"),
		field.JSON("periods", []map[string]any{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}).
			Comment("[{start_time,end_time,multiplier}]，秒级左闭右开，兼容 HH:mm。"),
		field.JSON("exclude_dates", []string{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}).
			Comment("这些日期（YYYY-MM-DD，按 timezone 的本地日期）全天按平时：法定节假日。"),
	}
}

func (ModelCatalogTimePricing) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("entry_id").Unique(),
	}
}
