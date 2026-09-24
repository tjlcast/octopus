package op

import (
	"testing"

	"github.com/bestruirui/octopus/internal/model"
)

func TestTrafficControlCheckBodyCombinesClauses(t *testing.T) {
	trafficControlCache.Lock()
	trafficControlCache.rules = []model.TrafficControlRule{
		{
			ID:         7,
			Name:       "require trace id",
			Enabled:    true,
			MatchType:  model.TrafficControlMatchTypeBody,
			ActionType: model.TrafficControlActionTypeFastFail,
			MatchConfig: model.TrafficControlMatchConfig{
				BodyClauses: []model.TrafficControlBodyClause{
					{Keyword: "trace_id"},
					{Keyword: "tenant_id", Operator: "and"},
					{Keyword: "debug", Operator: "and", Not: true},
				},
			},
			ActionConfig: model.TrafficControlActionConfig{
				StatusCode: 400,
				Message:    "missing trace id",
			},
		},
	}
	trafficControlCache.Unlock()
	t.Cleanup(func() {
		trafficControlCache.Lock()
		trafficControlCache.rules = nil
		trafficControlCache.Unlock()
	})

	if decision := TrafficControlCheckBody([]byte(`{"trace_id":"abc","tenant_id":"octopus"}`)); decision.Blocked {
		t.Fatalf("expected request matching all clauses to pass, got blocked by %q", decision.RuleName)
	}

	decision := TrafficControlCheckBody([]byte(`{"trace_id":"abc"}`))
	if !decision.Blocked {
		t.Fatal("expected request missing required clause to be blocked")
	}
	if decision.RuleID != 7 || decision.StatusCode != 400 || decision.Message != "missing trace id" {
		t.Fatalf("unexpected decision: %+v", decision)
	}

	if decision := TrafficControlCheckBody([]byte(`{"trace_id":"abc","tenant_id":"octopus","debug":true}`)); !decision.Blocked {
		t.Fatal("expected request matching negated clause to be blocked")
	}
}

func TestTrafficControlCheckBodyCombinesClausesWithOr(t *testing.T) {
	trafficControlCache.Lock()
	trafficControlCache.rules = []model.TrafficControlRule{
		{
			ID:         8,
			Name:       "require user marker",
			Enabled:    true,
			MatchType:  model.TrafficControlMatchTypeBody,
			ActionType: model.TrafficControlActionTypeFastFail,
			MatchConfig: model.TrafficControlMatchConfig{
				BodyClauses: []model.TrafficControlBodyClause{
					{Keyword: "user_id"},
					{Keyword: "account_id", Operator: "or"},
				},
			},
		},
	}
	trafficControlCache.Unlock()
	t.Cleanup(func() {
		trafficControlCache.Lock()
		trafficControlCache.rules = nil
		trafficControlCache.Unlock()
	})

	if decision := TrafficControlCheckBody([]byte(`{"account_id":"abc"}`)); decision.Blocked {
		t.Fatalf("expected request matching an OR clause to pass, got blocked by %q", decision.RuleName)
	}
	if decision := TrafficControlCheckBody([]byte(`{"message":"hello"}`)); !decision.Blocked {
		t.Fatal("expected request missing all OR clauses to be blocked")
	}
}

func TestTrafficControlCheckBodyKeepsLegacyFields(t *testing.T) {
	trafficControlCache.Lock()
	trafficControlCache.rules = []model.TrafficControlRule{
		{
			ID:         10,
			Name:       "require legacy body",
			Enabled:    true,
			MatchType:  model.TrafficControlMatchTypeBody,
			ActionType: model.TrafficControlActionTypeFastFail,
			MatchConfig: model.TrafficControlMatchConfig{
				BodyKeywords: []string{"trace_id", "tenant_id"},
				Mode:         "and",
			},
		},
	}
	trafficControlCache.Unlock()
	t.Cleanup(func() {
		trafficControlCache.Lock()
		trafficControlCache.rules = nil
		trafficControlCache.Unlock()
	})

	if decision := TrafficControlCheckBody([]byte(`{"trace_id":"abc","tenant_id":"octopus"}`)); decision.Blocked {
		t.Fatalf("expected legacy keywords to pass, got blocked by %q", decision.RuleName)
	}
	if decision := TrafficControlCheckBody([]byte(`{"trace_id":"abc"}`)); !decision.Blocked {
		t.Fatal("expected legacy keywords missing one value to be blocked")
	}
}
