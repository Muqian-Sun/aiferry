package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ModelCatalogEntry 是「平台上有哪些模型」的权威条目。
//
// 每条记录描述一个模型标识及其基准价卡：价格字段与 service.ModelPricing 一一对齐，
// 可空表示「未配置」，计费时回退到价格文件 / 硬编码兜底价。
//
// managed_by 区分条目由谁维护：
//   - seed：由 model-catalog 播种器从价格文件 + 硬编码兜底价生成，重复播种会刷新；
//   - admin：管理员改过，播种器永不覆盖。
type ModelCatalogEntry struct {
	ent.Schema
}

func (ModelCatalogEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "model_catalog_entries"},
	}
}

func (ModelCatalogEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ModelCatalogEntry) Fields() []ent.Field {
	return []ent.Field{
		field.String("model_id").
			MaxLen(200).
			NotEmpty().
			Comment("Canonical model identifier; unique case-insensitively."),
		field.String("display_name").
			MaxLen(200).
			Default("").
			Comment("Human-facing name; empty means show model_id."),
		field.String("vendor").
			MaxLen(50).
			Default("").
			Comment("Vendor that owns the model (anthropic/openai/gemini/xai/...); empty means unknown."),
		field.JSON("protocols", []string{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}).
			Comment("Upstream protocols the model speaks: anthropic/chat_completions/responses/gemini."),
		field.String("billing_mode").
			MaxLen(20).
			Default("token").
			Comment("token / per_request / image / video."),
		field.String("status").
			MaxLen(20).
			Default("listed").
			Comment("listed = 上架（用户可见且可调用）; unlisted = 下架。"),
		field.String("managed_by").
			MaxLen(20).
			Default("seed").
			Comment("seed = 播种器维护，可被重新播种刷新; admin = 管理员维护，播种器不再覆盖。"),

		// 基准价（USD per token），nil 表示未配置。
		modelCatalogPriceField("input_price"),
		modelCatalogPriceField("output_price"),
		modelCatalogPriceField("cache_write_price"),
		modelCatalogPriceField("cache_write_1h_price"),
		modelCatalogPriceField("cache_read_price"),
		modelCatalogPriceField("image_input_price"),
		modelCatalogPriceField("image_output_price"),
		modelCatalogPriceField("image_cache_read_price"),
		// 音频输入 / 输出 token 价；nil 时音频 token 按文本输入 / 输出价计。
		modelCatalogPriceField("audio_input_price"),
		modelCatalogPriceField("audio_output_price"),

		// 按次 / 图片 / 视频计费的默认单价。
		modelCatalogPriceField("per_request_price"),
		// 模型内置搜索每次调用价（alpha search 用；未配则用内置单价）。
		modelCatalogPriceField("search_price_per_call"),

		modelCatalogMultiplierField("max_reasoning_effort_multiplier"),

		field.Text("notes").
			Optional().
			Nillable(),
	}
}

// Edges 声明条目绑定的资源（账号），经 model_catalog_bindings 中间表。
func (ModelCatalogEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("accounts", Account.Type).
			Through("bindings", ModelCatalogBinding.Type),
		// subscription_plans: 条目进了哪些套餐的模型集，经 subscription_plan_models 中间表
		edge.From("subscription_plans", SubscriptionPlan.Type).
			Ref("catalog_entries").
			Through("plan_models", SubscriptionPlanModel.Type),
	}
}

func (ModelCatalogEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("vendor"),
		index.Fields("status"),
		index.Fields("managed_by"),
	}
}

// modelCatalogPriceField 声明一个可空的 USD/token 价格列。
// decimal(20,12) 与 channel_model_pricing 的价格列同精度：最小可表示 1e-12 USD/token，
// 即 $0.000001 per MTok，低于任何真实价卡的精度需求。
func modelCatalogPriceField(name string) ent.Field {
	return field.Float(name).
		Optional().
		Nillable().
		SchemaType(map[string]string{dialect.Postgres: "decimal(20,12)"})
}

// modelCatalogMultiplierField 声明一个可空的倍率列。
func modelCatalogMultiplierField(name string) ent.Field {
	return field.Float(name).
		Optional().
		Nillable().
		SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"})
}
