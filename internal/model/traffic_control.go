package model

import "time"

type TrafficControlMatchType string
type TrafficControlActionType string

const (
	TrafficControlMatchTypeIP        TrafficControlMatchType = "ip"
	TrafficControlMatchTypePath      TrafficControlMatchType = "path"
	TrafficControlMatchTypeBody      TrafficControlMatchType = "body"
	TrafficControlMatchTypeHeader    TrafficControlMatchType = "header"
	TrafficControlMatchTypeComposite TrafficControlMatchType = "composite"

	TrafficControlActionTypeFastFail    TrafficControlActionType = "fast_fail"
	TrafficControlActionTypeConcurrency TrafficControlActionType = "concurrency"
)

type TrafficControlMatchConfig struct {
	IPs          []string                   `json:"ips,omitempty"`
	Paths        []string                   `json:"paths,omitempty"`
	Headers      []string                   `json:"headers,omitempty"`
	Body         string                     `json:"body,omitempty"`
	BodyKeywords []string                   `json:"body_keywords,omitempty"`
	BodyClauses  []TrafficControlBodyClause `json:"body_clauses,omitempty"`
	Mode         string                     `json:"mode,omitempty"`
}

type TrafficControlBodyClause struct {
	Keyword  string `json:"keyword,omitempty"`
	Operator string `json:"operator,omitempty"`
	Not      bool   `json:"not,omitempty"`
}

type TrafficControlActionConfig struct {
	StatusCode int    `json:"status_code,omitempty"`
	Message    string `json:"message,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	WindowSec  int    `json:"window_sec,omitempty"`
}

type TrafficControlRule struct {
	ID           int                        `json:"id" gorm:"primaryKey"`
	Name         string                     `json:"name" gorm:"not null"`
	Description  string                     `json:"description,omitempty"`
	Enabled      bool                       `json:"enabled" gorm:"default:true"`
	Priority     int                        `json:"priority" gorm:"default:100;index"`
	MatchType    TrafficControlMatchType    `json:"match_type" gorm:"type:varchar(32);default:ip;index"`
	MatchConfig  TrafficControlMatchConfig  `json:"match_config" gorm:"serializer:json"`
	ActionType   TrafficControlActionType   `json:"action_type" gorm:"type:varchar(32);default:fast_fail;index"`
	ActionConfig TrafficControlActionConfig `json:"action_config" gorm:"serializer:json"`
	CreatedAt    time.Time                  `json:"created_at"`
	UpdatedAt    time.Time                  `json:"updated_at"`
}
