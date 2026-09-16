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

// Upstream stores an administrator-managed compatible relay endpoint.
type Upstream struct {
	ent.Schema
}

func (Upstream) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "upstreams"}}
}

func (Upstream) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}, mixins.SoftDeleteMixin{}}
}

func (Upstream) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(100).NotEmpty(),
		field.Int("sort_code").Default(0),
		field.String("kind").MaxLen(20).NotEmpty(),
		field.String("base_url").MaxLen(500).NotEmpty(),
		field.String("token_encrypted").Sensitive(),
		field.String("refresh_token_encrypted").Optional().Nillable().Sensitive(),
		field.String("password_encrypted").Optional().Sensitive(),
		field.Time("token_expires_at").Optional().Nillable(),
		field.String("login_identifier").Optional().Nillable().MaxLen(200),
		field.String("remote_user_id").Optional().Nillable().MaxLen(100),
		field.JSON("balance_snapshot", map[string]any{}).Default(map[string]any{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("group_snapshot", map[string]any{}).Default(map[string]any{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("notes").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Bool("enabled").Default(true),
		field.Time("last_checked_at").Optional().Nillable(),
		field.String("last_error").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int64("created_by"),
	}
}

func (Upstream) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("kind"),
		index.Fields("enabled"),
		index.Fields("last_checked_at"),
	}
}
