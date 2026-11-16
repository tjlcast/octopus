package op

import (
	"fmt"
	"strings"

	"github.com/bestruirui/octopus/internal/model"
	"gorm.io/gorm"
)

// RelayLogFilter describes optional, inclusive conditions for the log list.
type RelayLogFilter struct {
	StartTime, EndTime                *int
	MinInputTokens, MaxInputTokens    *int
	MinOutputTokens, MaxOutputTokens  *int
	MinFtut, MaxFtut                  *int
	MinUseTime, MaxUseTime            *int
	ChannelID                         *int
	RequestModelName, ActualModelName string
	Status                            string
	Keyword                           string
	KeywordNot                        bool
}

func (f RelayLogFilter) Validate() error {
	for _, pair := range [][2]*int{
		{f.StartTime, f.EndTime},
		{f.MinInputTokens, f.MaxInputTokens},
		{f.MinOutputTokens, f.MaxOutputTokens},
		{f.MinFtut, f.MaxFtut},
		{f.MinUseTime, f.MaxUseTime},
	} {
		if pair[0] != nil && pair[1] != nil && *pair[0] > *pair[1] {
			return fmt.Errorf("minimum value cannot exceed maximum value")
		}
	}
	return nil
}

func (f RelayLogFilter) matches(log model.RelayLog) bool {
	if (f.StartTime != nil && log.Time < int64(*f.StartTime)) ||
		(f.EndTime != nil && log.Time > int64(*f.EndTime)) ||
		(f.MinInputTokens != nil && log.InputTokens < *f.MinInputTokens) ||
		(f.MaxInputTokens != nil && log.InputTokens > *f.MaxInputTokens) ||
		(f.MinOutputTokens != nil && log.OutputTokens < *f.MinOutputTokens) ||
		(f.MaxOutputTokens != nil && log.OutputTokens > *f.MaxOutputTokens) ||
		(f.MinFtut != nil && log.Ftut < *f.MinFtut) ||
		(f.MaxFtut != nil && log.Ftut > *f.MaxFtut) ||
		(f.MinUseTime != nil && log.UseTime < *f.MinUseTime) ||
		(f.MaxUseTime != nil && log.UseTime > *f.MaxUseTime) ||
		(f.ChannelID != nil && log.ChannelId != *f.ChannelID) ||
		(f.RequestModelName != "" && log.RequestModelName != f.RequestModelName) ||
		(f.ActualModelName != "" && log.ActualModelName != f.ActualModelName) {
		return false
	}
	if f.Keyword != "" {
		keyword := strings.ToLower(f.Keyword)
		matched := strings.Contains(strings.ToLower(log.RequestContent), keyword) ||
			strings.Contains(strings.ToLower(log.ResponseContent), keyword) ||
			strings.Contains(strings.ToLower(log.Error), keyword)
		if matched == f.KeywordNot {
			return false
		}
	}
	return (f.Status != "success" || log.Error == "") && (f.Status != "failed" || log.Error != "")
}

func (f RelayLogFilter) apply(query *gorm.DB) *gorm.DB {
	for _, condition := range []struct {
		value *int
		sql   string
	}{
		{f.StartTime, "time >= ?"}, {f.EndTime, "time <= ?"},
		{f.MinInputTokens, "input_tokens >= ?"}, {f.MaxInputTokens, "input_tokens <= ?"},
		{f.MinOutputTokens, "output_tokens >= ?"}, {f.MaxOutputTokens, "output_tokens <= ?"},
		{f.MinFtut, "ftut >= ?"}, {f.MaxFtut, "ftut <= ?"},
		{f.MinUseTime, "use_time >= ?"}, {f.MaxUseTime, "use_time <= ?"},
		{f.ChannelID, "channel_id = ?"},
	} {
		if condition.value != nil {
			query = query.Where(condition.sql, *condition.value)
		}
	}
	if f.RequestModelName != "" {
		query = query.Where("request_model_name = ?", f.RequestModelName)
	}
	if f.ActualModelName != "" {
		query = query.Where("actual_model_name = ?", f.ActualModelName)
	}
	if f.Keyword != "" {
		pattern := "%" + escapeLikeKeyword(strings.ToLower(f.Keyword)) + "%"
		condition := "(LOWER(request_content) LIKE ? ESCAPE '\\' OR LOWER(response_content) LIKE ? ESCAPE '\\' OR LOWER(error) LIKE ? ESCAPE '\\')"
		if f.KeywordNot {
			query = query.Where("NOT "+condition, pattern, pattern, pattern)
		} else {
			query = query.Where(condition, pattern, pattern, pattern)
		}
	}
	if f.Status == "success" {
		query = query.Where("error = ''")
	} else if f.Status == "failed" {
		query = query.Where("error <> ''")
	}
	return query
}

func escapeLikeKeyword(keyword string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(keyword)
}
