package mock

import (
	"encoding/json"
	"strings"

	"api-mocker/models"
)

type Matcher struct{}

func NewMatcher() *Matcher {
	return &Matcher{}
}

func (m *Matcher) Match(conditions json.RawMessage, c ConditionContext) bool {
	if len(conditions) == 0 || string(conditions) == "[]" || string(conditions) == "null" {
		return false
	}

	var rules []models.ConditionRule
	if err := json.Unmarshal(conditions, &rules); err != nil {
		return false
	}

	for _, rule := range rules {
		if !m.matchRule(rule, c) {
			return false
		}
	}
	return true
}

func (m *Matcher) matchRule(rule models.ConditionRule, c ConditionContext) bool {
	var actual string

	switch strings.ToLower(rule.In) {
	case "query":
		actual = c.Query[rule.Field]
	case "header":
		actual = c.Headers[rule.Field]
	case "path":
		actual = c.PathParams[rule.Field]
	case "body":
		actual = c.Body[rule.Field]
	default:
		actual = c.Query[rule.Field]
	}

	switch strings.ToLower(rule.Operator) {
	case "eq", "==", "equals":
		return actual == rule.Value
	case "neq", "!=", "notequals":
		return actual != rule.Value
	case "contains":
		return strings.Contains(actual, rule.Value)
	case "startswith":
		return strings.HasPrefix(actual, rule.Value)
	case "endswith":
		return strings.HasSuffix(actual, rule.Value)
	case "exists":
		return actual != ""
	case "empty":
		return actual == ""
	default:
		return actual == rule.Value
	}
}

type ConditionContext struct {
	Query      map[string]string
	Headers    map[string]string
	PathParams map[string]string
	Body       map[string]string
}
