package handler

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

func TestPlanCatalogForResponseIncludesHistoricalLocalizedContent(t *testing.T) {
	plans := []*dbent.SubscriptionPlan{{
		ID:            2,
		GroupID:       3,
		Name:          "Codex 轻量版",
		NameEn:        "Codex Starter",
		Description:   "适合轻度使用",
		DescriptionEn: "For light usage",
		Features:      "$35 标准用量额度\n每日最高 $20",
		FeaturesEn:    "$35 standard usage quota\nUp to $20 per day",
		ForSale:       false,
	}}

	got := planCatalogForResponse(plans)
	if len(got) != 1 {
		t.Fatalf("expected one catalog item, got %d", len(got))
	}
	if got[0].NameEN != "Codex Starter" || got[0].DescriptionEN != "For light usage" {
		t.Fatalf("expected localized catalog content, got %#v", got[0])
	}
	if len(got[0].FeaturesEN) != 2 || got[0].FeaturesEN[1] != "Up to $20 per day" {
		t.Fatalf("expected localized features to be parsed, got %#v", got[0].FeaturesEN)
	}
}
