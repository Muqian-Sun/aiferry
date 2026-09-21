package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// SubscriptionPlanModel 是套餐的模型集成员：订阅 key 只能调用套餐里的目录条目。
// 复合主键只对 edge schema 生效，所以它是 SubscriptionPlan.catalog_entries 的 Through 类型（同 ModelCatalogBinding）。
type SubscriptionPlanModel struct {
	ent.Schema
}

func (SubscriptionPlanModel) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "subscription_plan_models"},
		// 复合主键：(plan_id, entry_id)。
		field.ID("plan_id", "entry_id"),
	}
}

func (SubscriptionPlanModel) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("plan_id"),
		field.Int64("entry_id"),
	}
}

func (SubscriptionPlanModel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("plan", SubscriptionPlan.Type).
			Unique().
			Required().
			Field("plan_id"),
		edge.To("entry", ModelCatalogEntry.Type).
			Unique().
			Required().
			Field("entry_id"),
	}
}

func (SubscriptionPlanModel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("entry_id"),
	}
}
