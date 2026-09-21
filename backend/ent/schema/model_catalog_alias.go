package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ModelCatalogAlias 把一个别名指向目录里的某个模型标识。
//
// 别名可以是精确名（"claude-3-5-sonnet-20241022"），也可以是尾部通配的前缀模式
// （"claude-3-5-sonnet-*"）。别名全局唯一（大小写不敏感），所以「同一个别名指向
// 两个模型」在库层就不可能发生。
type ModelCatalogAlias struct {
	ent.Schema
}

func (ModelCatalogAlias) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "model_catalog_aliases"},
	}
}

func (ModelCatalogAlias) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (ModelCatalogAlias) Fields() []ent.Field {
	return []ent.Field{
		field.String("alias").
			MaxLen(200).
			NotEmpty().
			Comment("别名，尾部 * 表示前缀匹配；大小写不敏感唯一。"),
		field.Int64("entry_id").
			Comment("指向 model_catalog_entries.id。"),
		field.String("source").
			MaxLen(30).
			Default("manual").
			Comment("别名来源：manual / seed。"),
		field.Text("notes").
			Optional().
			Nillable(),
	}
}

func (ModelCatalogAlias) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("entry_id"),
		index.Fields("source"),
	}
}
