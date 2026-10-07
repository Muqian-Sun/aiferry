package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ModelCatalogBinding 是目录条目与资源（账号）的绑定（承接关系）：条目上架后由这些账号承接请求。
// 每条承接关系带这个渠道给这个模型的上游价：渠道成本 = 用量 × 上游价。输入 / 输出必填，
// 官方价有的缓存项由服务层要求必填；可按 Token 分段（price_intervals）。
// upstream_model 是转发时这个渠道认的模型名（目录标识 → 上游名只转换这一次）。
type ModelCatalogBinding struct {
	ent.Schema
}

func (ModelCatalogBinding) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "model_catalog_bindings"},
		// 复合主键：(entry_id, account_id)。
		field.ID("entry_id", "account_id"),
	}
}

func (ModelCatalogBinding) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("entry_id"),
		field.Int64("account_id"),
		field.Float("input_price").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,12)"}),
		field.Float("output_price").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,12)"}),
		field.Float("cache_write_price").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,12)"}),
		field.Float("cache_write_1h_price").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,12)"}),
		field.Float("cache_read_price").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,12)"}),
		// 联网搜索的上游价（USD）：每次 web 搜索、每条 X 帖子、每个 X 主页；未填 = 上游不收搜索费。
		field.Float("search_price_per_call").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,12)"}),
		field.Float("x_post_price").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,12)"}),
		field.Float("x_user_price").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,12)"}),
		// max_reasoning_effort_multiplier 上游在最高推理档（effort = max）整单乘的倍数；NULL = 跟官方。
		field.Float("max_reasoning_effort_multiplier").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,4)"}),
		// upstream_model 这个渠道给这个模型用的上游模型名，空串 = 与目录标识同名（按目录模型精确对应，不支持通配）。
		field.String("upstream_model").
			MaxLen(255).
			Default(""),
		field.JSON("price_intervals", []domain.PriceSegment{}).
			Default([]domain.PriceSegment{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		// time_pricing 上游忙闲时：按请求时刻整单 × 倍率算渠道成本；NULL = 上游不分忙闲时。
		field.JSON("time_pricing", &domain.TimePricingSpec{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (ModelCatalogBinding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("entry", ModelCatalogEntry.Type).
			Unique().
			Required().
			Field("entry_id"),
		edge.To("account", Account.Type).
			Unique().
			Required().
			Field("account_id"),
	}
}

func (ModelCatalogBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("account_id"),
	}
}
