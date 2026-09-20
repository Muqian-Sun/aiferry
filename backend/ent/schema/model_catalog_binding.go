package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ModelCatalogBinding 是目录条目与资源（账号）的绑定：条目上架后由这些账号承接请求。
// priority 为空时沿用账号自身的 priority。
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
		field.Int("priority").
			Optional().
			Nillable(),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
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
