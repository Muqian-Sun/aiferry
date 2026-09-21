package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ModelCatalogPriceInterval 是目录条目的分档定价，语义与 channel_pricing_intervals 相同：
//   - token 计费：按 context token 数分档，区间左开右闭 (min_tokens, max_tokens]；
//   - per_request / image / video 计费：按 tier_label 分层，min/max 不参与匹配。
//
// 绝对价优先于倍率：同一档里既配了 input_price 又配了 input_multiplier 时用价格。
type ModelCatalogPriceInterval struct {
	ent.Schema
}

func (ModelCatalogPriceInterval) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "model_catalog_price_intervals"},
	}
}

func (ModelCatalogPriceInterval) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ModelCatalogPriceInterval) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("entry_id").
			Comment("指向 model_catalog_entries.id。"),
		field.Int("min_tokens").
			Default(0),
		field.Int("max_tokens").
			Optional().
			Nillable().
			Comment("nil 表示无上界；无上界区间必须是最后一档。"),
		field.String("tier_label").
			MaxLen(50).
			Default("").
			Comment("按次 / 图片 / 视频计费的档位标签，如 1K / 2K / HD。"),

		modelCatalogPriceField("input_price"),
		modelCatalogPriceField("output_price"),
		modelCatalogPriceField("cache_write_price"),
		modelCatalogPriceField("cache_write_1h_price"),
		modelCatalogPriceField("cache_read_price"),
		modelCatalogPriceField("per_request_price"),

		modelCatalogMultiplierField("input_multiplier"),
		modelCatalogMultiplierField("output_multiplier"),
		modelCatalogMultiplierField("cache_write_multiplier"),
		modelCatalogMultiplierField("cache_read_multiplier"),

		field.Int("sort_order").
			Default(0),
	}
}

func (ModelCatalogPriceInterval) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("entry_id", "sort_order"),
	}
}
