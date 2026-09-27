package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UpstreamResourceAccount records one synchronized account per resource and platform.
type UpstreamResourceAccount struct {
	ent.Schema
}

func (UpstreamResourceAccount) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "upstream_resource_accounts"}}
}

func (UpstreamResourceAccount) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("resource_id").Positive(),
		field.String("platform").MaxLen(50).NotEmpty(),
		field.Int64("account_id").Positive(),
		field.Float("rate_multiplier"),
		field.Time("synced_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (UpstreamResourceAccount) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("resource_id", "platform").Unique(),
		index.Fields("account_id").Unique(),
	}
}
