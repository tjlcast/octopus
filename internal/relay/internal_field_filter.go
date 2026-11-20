package relay

import (
	"sort"
	"strings"

	"github.com/bestruirui/octopus/internal/transformer/model"
)

// internalFieldKeyword is a provider-specific marker embedded in otherwise
// ordinary text. Only the marker is removed; all surrounding text and markup
// must be preserved exactly.
const defaultInternalFieldKeyword = "｜｜DSML｜｜ "

// internalFieldFilter is scoped to one relay attempt. Stream state must not be
// shared between requests, because a provider can split the keyword across any
// number of SSE events.
type internalFieldFilter struct {
	enabled  bool
	keywords []string
	streams  map[int]*internalFieldStreamState
}

type internalFieldStreamState struct {
	pending string
}

func newInternalFieldFilter(enabled bool, rawKeywords *string) *internalFieldFilter {
	return &internalFieldFilter{
		enabled:  enabled,
		keywords: parseInternalFieldKeywords(rawKeywords),
		streams:  make(map[int]*internalFieldStreamState),
	}
}

func parseInternalFieldKeywords(rawKeywords *string) []string {
	keywords := make([]string, 0)
	seen := make(map[string]struct{})
	if rawKeywords != nil {
		for _, line := range strings.Split(*rawKeywords, "\n") {
			// Newlines delimit entries, but spaces are part of a keyword. In
			// particular, some providers emit a marker followed by a space.
			keyword := strings.TrimSuffix(line, "\r")
			if keyword == "" {
				continue
			}
			if _, exists := seen[keyword]; !exists {
				seen[keyword] = struct{}{}
				keywords = append(keywords, keyword)
			}
		}
	}
	if len(keywords) == 0 {
		keywords = append(keywords, defaultInternalFieldKeyword)
	}
	// Prefer a longer keyword when two entries begin at the same offset.
	sort.SliceStable(keywords, func(i, j int) bool { return len(keywords[i]) > len(keywords[j]) })
	return keywords
}

func (f *internalFieldFilter) FilterResponse(response *model.InternalLLMResponse) {
	if !f.enabled || response == nil {
		return
	}

	for i := range response.Choices {
		filterMessageInternalFields(response.Choices[i].Message, f.keywords)
		filterMessageInternalFields(response.Choices[i].Delta, f.keywords)
	}
}

func filterMessageInternalFields(message *model.Message, keywords []string) {
	if message == nil {
		return
	}
	if message.Content.Content != nil {
		filtered := removeInternalFieldKeywords(*message.Content.Content, keywords)
		message.Content.Content = &filtered
	}
	for i := range message.Content.MultipleContent {
		part := &message.Content.MultipleContent[i]
		if part.Type == "text" && part.Text != nil {
			filtered := removeInternalFieldKeywords(*part.Text, keywords)
			part.Text = &filtered
		}
	}
}

// FilterStream filters textual delta content in place. On [DONE], it emits any
// ordinary text held only to recognize a keyword split across SSE events, then
// emits [DONE].
func (f *internalFieldFilter) FilterStream(stream *model.InternalLLMResponse) []*model.InternalLLMResponse {
	if !f.enabled || stream == nil {
		return []*model.InternalLLMResponse{stream}
	}

	if stream.Object == "[DONE]" {
		flushed := f.flush(stream)
		return append(flushed, stream)
	}

	for i := range stream.Choices {
		choice := &stream.Choices[i]
		if choice.Delta == nil {
			continue
		}
		filterStreamMessage(f.state(choice.Index), choice.Delta, f.keywords)
	}
	return []*model.InternalLLMResponse{stream}
}

func (f *internalFieldFilter) state(index int) *internalFieldStreamState {
	state := f.streams[index]
	if state == nil {
		state = &internalFieldStreamState{}
		f.streams[index] = state
	}
	return state
}

func filterStreamMessage(state *internalFieldStreamState, message *model.Message, keywords []string) {
	if message.Content.Content != nil {
		filtered := state.filter(*message.Content.Content, keywords)
		message.Content.Content = &filtered
	}

	// Current outbound transformers produce text deltas through Content.Content.
	// Filter text parts too so a future transformer cannot bypass this safeguard.
	for i := range message.Content.MultipleContent {
		part := &message.Content.MultipleContent[i]
		if part.Type == "text" && part.Text != nil {
			filtered := state.filter(*part.Text, keywords)
			part.Text = &filtered
		}
	}
}

func (s *internalFieldStreamState) filter(text string, keywords []string) string {
	input := s.pending + text
	s.pending = ""
	var output strings.Builder
	for {
		matchStart, matchedKeyword := nextKeywordMatch(input, keywords)
		if matchStart < 0 {
			s.pending = suffixThatPrefixesAny(input, keywords)
			output.WriteString(input[:len(input)-len(s.pending)])
			return output.String()
		}
		output.WriteString(input[:matchStart])
		input = input[matchStart+len(matchedKeyword):]
	}
}

func (f *internalFieldFilter) flush(done *model.InternalLLMResponse) []*model.InternalLLMResponse {
	choices := make([]model.Choice, 0, len(f.streams))
	for index, state := range f.streams {
		if state.pending == "" {
			continue
		}
		text := state.pending
		choices = append(choices, model.Choice{
			Index: index,
			Delta: &model.Message{Content: model.MessageContent{Content: &text}},
		})
	}
	f.streams = make(map[int]*internalFieldStreamState)
	if len(choices) == 0 {
		return nil
	}

	return []*model.InternalLLMResponse{{
		ID:      done.ID,
		Object:  "chat.completion.chunk",
		Created: done.Created,
		Model:   done.Model,
		Choices: choices,
	}}
}

// suffixThatPrefixes returns the longest suffix of value that could be the
// beginning of marker. It lets the stream filter recognize a keyword split
// across SSE events without delaying unrelated text.
func removeInternalFieldKeywords(text string, keywords []string) string {
	for _, keyword := range keywords {
		text = strings.ReplaceAll(text, keyword, "")
	}
	return text
}

func nextKeywordMatch(value string, keywords []string) (int, string) {
	bestIndex := -1
	bestKeyword := ""
	for _, keyword := range keywords {
		index := strings.Index(value, keyword)
		if index >= 0 && (bestIndex < 0 || index < bestIndex) {
			bestIndex = index
			bestKeyword = keyword
		}
	}
	return bestIndex, bestKeyword
}

func suffixThatPrefixesAny(value string, keywords []string) string {
	longest := ""
	for _, keyword := range keywords {
		candidate := suffixThatPrefixes(value, keyword)
		if len(candidate) > len(longest) {
			longest = candidate
		}
	}
	return longest
}

func suffixThatPrefixes(value, marker string) string {
	max := len(value)
	if max >= len(marker) {
		max = len(marker) - 1
	}
	for n := max; n > 0; n-- {
		if strings.HasSuffix(value, marker[:n]) {
			return value[len(value)-n:]
		}
	}
	return ""
}
