package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type GrowthEvent struct {
	ent.Schema
}

func (GrowthEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "growth_events"}}
}

func (GrowthEvent) Fields() []ent.Field {
	return []ent.Field{
		field.String("event_type").MaxLen(64),
		field.String("campaign_id").MaxLen(100).Default(""),
		field.String("platform").MaxLen(32).Default(""),
		field.String("app_version").MaxLen(32).Default(""),
		field.String("session_hash").MaxLen(64).Default(""),
		field.Int64("user_id").Optional().Nillable(),
		field.Int64("plan_id").Optional().Nillable(),
		field.String("payment_provider").MaxLen(32).Default(""),
		field.String("result").MaxLen(32).Default("success"),
		field.String("error_code").MaxLen(100).Default(""),
		field.Time("created_at").Default(time.Now).Immutable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (GrowthEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("event_type", "created_at"),
		index.Fields("campaign_id", "created_at"),
		index.Fields("session_hash"),
		index.Fields("user_id", "created_at"),
	}
}
