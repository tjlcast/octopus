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
		return trafficControlDecisionFromRule(rule)
	}

	return TrafficControlDecision{}
}

func TrafficControlCheckBody(body []byte) TrafficControlDecision {
	trafficControlCache.RLock()
	rules := append([]model.TrafficControlRule(nil), trafficControlCache.rules...)
	trafficControlCache.RUnlock()

	bodyText := string(body)
	for _, rule := range rules {
		if !rule.Enabled || rule.MatchType != model.TrafficControlMatchTypeBody {
			continue
		}
		if rule.ActionType != model.TrafficControlActionTypeFastFail {
			continue
		}
		clauses := trafficControlBodyClauses(rule.MatchConfig)
		if len(clauses) == 0 {
			continue
		}
		if trafficControlBodyMatch(bodyText, clauses) {
			continue
		}
		return trafficControlDecisionFromRule(rule)
	}

	return TrafficControlDecision{}
}

func trafficControlDecisionFromRule(rule model.TrafficControlRule) TrafficControlDecision {
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
	rule.MatchConfig.Body = strings.TrimSpace(rule.MatchConfig.Body)
	rule.MatchConfig.BodyKeywords = normalizeOrderedStringList(rule.MatchConfig.BodyKeywords)
	rule.MatchConfig.Mode = strings.ToLower(strings.TrimSpace(rule.MatchConfig.Mode))
	if rule.MatchConfig.Mode == "" && rule.MatchType == model.TrafficControlMatchTypeBody {
		rule.MatchConfig.Mode = "and"
	}
	rule.MatchConfig.BodyClauses = trafficControlBodyClauses(rule.MatchConfig)
	if len(rule.MatchConfig.BodyClauses) > 0 {
		keywords := make([]string, 0, len(rule.MatchConfig.BodyClauses))
		for _, clause := range rule.MatchConfig.BodyClauses {
			keywords = append(keywords, clause.Keyword)
		}
		rule.MatchConfig.BodyKeywords = keywords
		rule.MatchConfig.Body = rule.MatchConfig.BodyClauses[0].Keyword
	}
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
	case model.TrafficControlMatchTypeBody:
		if len(trafficControlBodyClauses(rule.MatchConfig)) == 0 {
			return fmt.Errorf("traffic control rule body keyword is required")
		}
	case model.TrafficControlMatchTypePath, model.TrafficControlMatchTypeHeader, model.TrafficControlMatchTypeComposite:
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

func trafficControlBodyClauses(config model.TrafficControlMatchConfig) []model.TrafficControlBodyClause {
	if len(config.BodyClauses) > 0 {
		clauses := make([]model.TrafficControlBodyClause, 0, len(config.BodyClauses))
		for _, clause := range config.BodyClauses {
			keyword := strings.TrimSpace(clause.Keyword)
			if keyword == "" {
				continue
			}
			operator := normalizeTrafficControlBodyOperator(clause.Operator)
			clauses = append(clauses, model.TrafficControlBodyClause{
				Keyword:  keyword,
				Operator: operator,
				Not:      clause.Not,
			})
		}
		return clauses
	}

	keywords := normalizeOrderedStringList(config.BodyKeywords)
	body := strings.TrimSpace(config.Body)
	if len(keywords) == 0 && body != "" {
		keywords = []string{body}
	}
	clauses := make([]model.TrafficControlBodyClause, 0, len(keywords))
	mode := strings.ToLower(strings.TrimSpace(config.Mode))
	for index, keyword := range keywords {
		operator := "and"
		if index > 0 && mode == "or" {
			operator = "or"
		}
		clauses = append(clauses, model.TrafficControlBodyClause{
			Keyword:  keyword,
			Operator: operator,
			Not:      mode == "not",
		})
	}
	return clauses
}

func normalizeTrafficControlBodyOperator(operator string) string {
	switch strings.ToLower(strings.TrimSpace(operator)) {
	case "or":
		return "or"
	default:
		return "and"
	}
}

func normalizeOrderedStringList(values []string) []string {
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
	return next
}

func trafficControlBodyMatch(body string, clauses []model.TrafficControlBodyClause) bool {
	result := trafficControlBodyClauseMatch(body, clauses[0])
	for index := 1; index < len(clauses); index++ {
		next := trafficControlBodyClauseMatch(body, clauses[index])
		if clauses[index].Operator == "or" {
			result = result || next
			continue
		}
		result = result && next
	}
	return result
}

func trafficControlBodyClauseMatch(body string, clause model.TrafficControlBodyClause) bool {
	matched := strings.Contains(body, clause.Keyword)
	if clause.Not {
		return !matched
	}
	return matched
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
