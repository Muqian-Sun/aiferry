package admin

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestSanitizeAdminPaymentOrderForResponseAddsCurrency(t *testing.T) {
	now := time.Now()
	order := &dbent.PaymentOrder{
		ID:          1,
		UserID:      2,
		Amount:      100,
		PayAmount:   108,
		FeeRate:     8,
		OutTradeNo:  "sub2_202606250001",
		PaymentType: "stripe",
		OrderType:   "subscription",
		Status:      "COMPLETED",
		ExpiresAt:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
		ProviderSnapshot: map[string]any{
			"schema_version": 2,
			"currency":       "USD",
		},
	}

	got := sanitizeAdminPaymentOrderForResponse(order)
	if got == nil {
		t.Fatal("expected sanitized order")
	}
	if got.Currency != "USD" {
		t.Fatalf("expected currency USD, got %q", got.Currency)
	}

	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal sanitized order: %v", err)
	}
	if strings.Contains(string(body), "provider_snapshot") {
		t.Fatalf("expected provider_snapshot to be omitted, got %s", string(body))
	}
}

func TestAdminSubscriptionPlansForResponseIncludesLimitsAndModels(t *testing.T) {
	weekly := 25.0
	now := time.Now()
	plans := []service.SubscriptionPlan{
		{
			ID:             11,
			Name:           "All models",
			Description:    "Composite access",
			Price:          19.99,
			Currency:       "CNY",
			ValidityDays:   30,
			ValidityUnit:   "days",
			Features:       "OpenAI\nClaude\nGemini\nGrok",
			ProductName:    "Sub2API",
			ForSale:        true,
			SortOrder:      1,
			WeeklyLimitUSD: &weekly,
			Models: []service.SubscriptionPlanModel{
				{EntryID: 199, ModelID: "gpt-5.6", DisplayName: "GPT-5.6"},
				{EntryID: 27, ModelID: "claude-sonnet-4-5", DisplayName: "Claude Sonnet 4.5"},
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	got := adminSubscriptionPlansForResponse(plans)

	if len(got) != 1 {
		t.Fatalf("expected one plan, got %d", len(got))
	}
	if got[0].WeeklyLimitUSD == nil || *got[0].WeeklyLimitUSD != weekly {
		t.Fatalf("expected weekly limit to be included, got %#v", got[0].WeeklyLimitUSD)
	}
	if got[0].DailyLimitUSD != nil || got[0].MonthlyLimitUSD != nil {
		t.Fatalf("unset limits must stay null, got %#v / %#v", got[0].DailyLimitUSD, got[0].MonthlyLimitUSD)
	}
	if len(got[0].EntryIDs) != 2 || got[0].EntryIDs[0] != 199 || got[0].EntryIDs[1] != 27 {
		t.Fatalf("expected entry ids in plan order, got %#v", got[0].EntryIDs)
	}
	if len(got[0].Models) != 2 || got[0].Models[1].DisplayName != "Claude Sonnet 4.5" {
		t.Fatalf("expected model names to be included, got %#v", got[0].Models)
	}
	// 投影必须保留套餐的全部售卖字段：currency 丢失曾导致编辑保存时
	// 静默清空套餐货币（PlanEditDialog 回传空串 → SetCurrency("")）。
	if got[0].Currency != "CNY" {
		t.Fatalf("expected currency to be preserved, got %q", got[0].Currency)
	}
	if !got[0].CreatedAt.Equal(now) || !got[0].UpdatedAt.Equal(now) {
		t.Fatalf("expected created_at/updated_at to be preserved, got %v / %v", got[0].CreatedAt, got[0].UpdatedAt)
	}
}
