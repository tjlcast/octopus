package op

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"sync"

	"github.com/bestruirui/octopus/internal/db"
	"github.com/bestruirui/octopus/internal/model"
)

type TrafficControlDecision struct {
	Blocked    bool
	RuleID     int
	RuleName   string
	StatusCode int
	Message    string
}

var trafficControlCache = struct {
	sync.RWMutex
	rules []model.TrafficControlRule
}{rules: make([]model.TrafficControlRule, 0)}

func TrafficControlRuleCreate(rule *model.TrafficControlRule, ctx context.Context) error {
	normalizeTrafficControlRule(rule)
	if err := validateTrafficControlRule(rule); err != nil {
		return err
	}
	if err := db.GetDB().WithContext(ctx).Create(rule).Error; err != nil {
		return fmt.Errorf("failed to create traffic control rule: %w", err)
	}
	return trafficControlRefreshCache(ctx)
}

func TrafficControlRuleUpdate(rule *model.TrafficControlRule, ctx context.Context) error {
	normalizeTrafficControlRule(rule)
	if err := validateTrafficControlRule(rule); err != nil {
		return err
	}
	var existing model.TrafficControlRule
	if err := db.GetDB().WithContext(ctx).First(&existing, rule.ID).Error; err != nil {
		return fmt.Errorf("traffic control rule not found")
	}
	rule.CreatedAt = existing.CreatedAt
	result := db.GetDB().WithContext(ctx).Save(rule)
	if result.Error != nil {
		return fmt.Errorf("failed to update traffic control rule: %w", result.Error)
	}
	return trafficControlRefreshCache(ctx)
}

func TrafficControlRuleList(ctx context.Context) ([]model.TrafficControlRule, error) {
	rules := make([]model.TrafficControlRule, 0)
	if err := db.GetDB().WithContext(ctx).Order("priority ASC").Order("id ASC").Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("failed to list traffic control rules: %w", err)
	}
	return rules, nil
}

func TrafficControlRuleDelete(id int, ctx context.Context) error {
	result := db.GetDB().WithContext(ctx).Delete(&model.TrafficControlRule{ID: id})
	if result.Error != nil {
		return fmt.Errorf("failed to delete traffic control rule: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("traffic control rule not found")
	}
	return trafficControlRefreshCache(ctx)
}

func TrafficControlCheckIP(ip string) TrafficControlDecision {
	trafficControlCache.RLock()
	rules := append([]model.TrafficControlRule(nil), trafficControlCache.rules...)
	trafficControlCache.RUnlock()

	for _, rule := range rules {
		if !rule.Enabled || rule.MatchType != model.TrafficControlMatchTypeIP {
			continue
		}
		if rule.ActionType != model.TrafficControlActionTypeFastFail {
			continue
		}
		if !trafficControlIPMatch(ip, rule.MatchConfig.IPs) {
			continue
		}
		statusCode := rule.ActionConfig.StatusCode
		if statusCode < 400 || statusCode > 599 {
			statusCode = 429
		}
		message := strings.TrimSpace(rule.ActionConfig.Message)
		if message == "" {
			message = "request blocked by traffic control"
		}
		return TrafficControlDecision{
			Blocked:    true,
			RuleID:     rule.ID,
			RuleName:   rule.Name,
			StatusCode: statusCode,
			Message:    message,
		}
	}

	return TrafficControlDecision{}
}

func trafficControlRefreshCache(ctx context.Context) error {
	rules := make([]model.TrafficControlRule, 0)
	if err := db.GetDB().WithContext(ctx).
		Where("enabled = ?", true).
		Order("priority ASC").
		Order("id ASC").
		Find(&rules).Error; err != nil {
		return err
	}

	trafficControlCache.Lock()
	trafficControlCache.rules = rules
	trafficControlCache.Unlock()
	return nil
}

func normalizeTrafficControlRule(rule *model.TrafficControlRule) {
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Description = strings.TrimSpace(rule.Description)
	if rule.MatchType == "" {
		rule.MatchType = model.TrafficControlMatchTypeIP
	}
	if rule.ActionType == "" {
		rule.ActionType = model.TrafficControlActionTypeFastFail
	}
	if rule.Priority == 0 {
		rule.Priority = 100
	}
	if rule.ActionConfig.StatusCode == 0 {
		rule.ActionConfig.StatusCode = 429
	}
	rule.MatchConfig.IPs = normalizeStringList(rule.MatchConfig.IPs)
	rule.MatchConfig.Paths = normalizeStringList(rule.MatchConfig.Paths)
	rule.MatchConfig.Headers = normalizeStringList(rule.MatchConfig.Headers)
}

func validateTrafficControlRule(rule *model.TrafficControlRule) error {
	if rule.Name == "" {
		return fmt.Errorf("traffic control rule name is required")
	}
	switch rule.MatchType {
	case model.TrafficControlMatchTypeIP:
		if len(rule.MatchConfig.IPs) == 0 {
			return fmt.Errorf("traffic control rule ip list is required")
		}
	case model.TrafficControlMatchTypePath, model.TrafficControlMatchTypeBody, model.TrafficControlMatchTypeHeader, model.TrafficControlMatchTypeComposite:
	default:
		return fmt.Errorf("unsupported traffic control match type: %s", rule.MatchType)
	}
	switch rule.ActionType {
	case model.TrafficControlActionTypeFastFail:
	case model.TrafficControlActionTypeConcurrency:
	default:
		return fmt.Errorf("unsupported traffic control action type: %s", rule.ActionType)
	}
	return nil
}

func normalizeStringList(values []string) []string {
	next := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		next = append(next, value)
	}
	sort.Strings(next)
	return next
}

func trafficControlIPMatch(ip string, rules []string) bool {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return false
	}

	for _, raw := range rules {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			prefix, err := netip.ParsePrefix(item)
			if err == nil && prefix.Contains(addr) {
				return true
			}
			continue
		}
		if strings.Contains(item, "-") {
			parts := strings.SplitN(item, "-", 2)
			start, startErr := netip.ParseAddr(strings.TrimSpace(parts[0]))
			end, endErr := netip.ParseAddr(strings.TrimSpace(parts[1]))
			if startErr == nil && endErr == nil && addr.Compare(start) >= 0 && addr.Compare(end) <= 0 {
				return true
			}
			continue
		}
		exact, err := netip.ParseAddr(item)
		if err == nil && exact == addr {
			return true
		}
	}

	return false
}
