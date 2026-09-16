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

// UpstreamResource stores an upstream key and the local sync mapping.
type UpstreamResource struct {
	ent.Schema
}

func (UpstreamResource) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "upstream_resources"}}
}

func (UpstreamResource) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}, mixins.SoftDeleteMixin{}}
}

func (UpstreamResource) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("upstream_id").Positive(),
		field.String("resource_type").MaxLen(20).NotEmpty(),
		field.String("remote_id").MaxLen(100).NotEmpty(),
		field.String("name").MaxLen(200),
		field.String("group_name").Optional().Nillable().MaxLen(200),
		field.String("key_encrypted").Sensitive(),
		field.JSON("models_snapshot", []string{}).Default([]string{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("models_fetched_at").Optional().Nillable(),
		field.Int64("synced_account_id").Optional().Nillable(),
		field.Float("synced_rate_multiplier").Optional().Nillable(),
		field.Time("synced_at").Optional().Nillable(),
		field.Bool("enabled").Default(true),
	}
}

func (UpstreamResource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("upstream_id"),
		index.Fields("upstream_id", "resource_type"),
		index.Fields("synced_account_id"),
		index.Fields("upstream_id", "resource_type", "remote_id").Unique(),
	}
}
