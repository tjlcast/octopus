package relay

import (
	"reflect"
	"testing"

	"github.com/bestruirui/octopus/internal/transformer/model"
)

func TestRemoveInternalFieldKeyword(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "between text", in: "before" + defaultInternalFieldKeyword + "after", want: "beforeafter"},
		{name: "multiple keywords", in: "<" + defaultInternalFieldKeyword + "calls><" + defaultInternalFieldKeyword + "invoke>", want: "<calls><invoke>"},
		{name: "ordinary text", in: "normal response", want: "normal response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeInternalFieldKeywords(tt.in, []string{defaultInternalFieldKeyword}); got != tt.want {
				t.Fatalf("removeInternalFieldKeywords() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInternalFieldFilterResponsePreservesOtherText(t *testing.T) {
	content := "<" + defaultInternalFieldKeyword + "calls><" + defaultInternalFieldKeyword + "parameter>胡总</" + defaultInternalFieldKeyword + "parameter>"
	part := "part" + defaultInternalFieldKeyword + " visible"
	toolArguments := "{\"value\":\"" + defaultInternalFieldKeyword + "\"}"
	response := &model.InternalLLMResponse{Choices: []model.Choice{{
		Message: &model.Message{
			Content: model.MessageContent{
				Content:         &content,
				MultipleContent: []model.MessageContentPart{{Type: "text", Text: &part}},
			},
			ToolCalls: []model.ToolCall{{Function: model.FunctionCall{Arguments: toolArguments}}},
		},
	}}}

	newInternalFieldFilter(true, nil).FilterResponse(response)
	message := response.Choices[0].Message
	if *message.Content.Content != "<calls><parameter>胡总</parameter>" || *message.Content.MultipleContent[0].Text != "part visible" {
		t.Fatalf("unexpected filtered message: %#v", message.Content)
	}
	if message.ToolCalls[0].Function.Arguments != toolArguments {
		t.Fatal("filter must not modify tool call arguments")
	}
}

func TestInternalFieldFilterStreamAcrossChunksAndFlush(t *testing.T) {
	filter := newInternalFieldFilter(true, nil)
	chunks := []string{
		"<｜｜DSML｜",
		"｜ calls><｜｜DSML｜｜ invoke name=\"lookup_person\"><｜｜DSML｜｜ parameter name=\"name\">胡总</｜｜DSML｜｜ parameter>",
		"<｜｜DSML｜｜",
		" parameter>",
		" </",
		"｜｜DSML",
		"｜｜ invoke></",
		"｜｜DSML",
		"｜｜ calls>",
	}
	var got string
	for _, text := range chunks {
		for _, stream := range filter.FilterStream(textChunk(0, text)) {
			got += streamText(stream)
		}
	}
	for _, stream := range filter.FilterStream(&model.InternalLLMResponse{Object: "[DONE]"}) {
		got += streamText(stream)
	}
	want := "<calls><invoke name=\"lookup_person\"><parameter name=\"name\">胡总</parameter><parameter> </invoke></calls>"
	if got != want {
		t.Fatalf("stream result = %q, want %q", got, want)
	}
}

func TestInternalFieldFilterDisabledDoesNotModifyResponse(t *testing.T) {
	text := defaultInternalFieldKeyword + "private"
	response := textChunk(0, text)
	filter := newInternalFieldFilter(false, nil)
	got := filter.FilterStream(response)
	if len(got) != 1 || !reflect.DeepEqual(got[0], response) || streamText(got[0]) != text {
		t.Fatal("disabled filter must leave stream unchanged")
	}
}

func TestInternalFieldFilterUsesConfiguredKeywords(t *testing.T) {
	keywords := "<internal-a>\n<internal-b>"
	filter := newInternalFieldFilter(true, &keywords)
	chunks := []string{"before<internal", "-a>middle<internal-b", ">after"}
	var got string
	for _, chunk := range chunks {
		for _, stream := range filter.FilterStream(textChunk(0, chunk)) {
			got += streamText(stream)
		}
	}
	for _, stream := range filter.FilterStream(&model.InternalLLMResponse{Object: "[DONE]"}) {
		got += streamText(stream)
	}
	if got != "beforemiddleafter" {
		t.Fatalf("configured keyword filtering = %q, want %q", got, "beforemiddleafter")
	}
}

func TestInternalFieldFilterPreservesWhitespaceInConfiguredKeywords(t *testing.T) {
	keywords := " lead\ntrail \nmid dle\n \n"
	filter := newInternalFieldFilter(true, &keywords)
	text := "x leadytrail zmid dleq r"
	for _, stream := range filter.FilterStream(textChunk(0, text)) {
		text = streamText(stream)
	}
	for _, stream := range filter.FilterStream(&model.InternalLLMResponse{Object: "[DONE]"}) {
		text += streamText(stream)
	}
	if text != "xyzqr" {
		t.Fatalf("whitespace-preserving filtering = %q, want %q", text, "xyzqr")
	}
}

func TestInternalFieldFilterFlushesIncompleteKeywordPrefixAtStreamEnd(t *testing.T) {
	filter := newInternalFieldFilter(true, nil)
	input := "asdfasdfasd｜｜DSML"
	var got string
	for _, stream := range filter.FilterStream(textChunk(0, input)) {
		got += streamText(stream)
	}
	for _, stream := range filter.FilterStream(&model.InternalLLMResponse{Object: "[DONE]"}) {
		got += streamText(stream)
	}
	if got != input {
		t.Fatalf("incomplete keyword prefix = %q, want %q", got, input)
	}
}

func textChunk(index int, text string) *model.InternalLLMResponse {
	return &model.InternalLLMResponse{
		Object: "chat.completion.chunk",
		Choices: []model.Choice{{
			Index: index,
			Delta: &model.Message{Content: model.MessageContent{Content: &text}},
		}},
	}
}

func streamText(stream *model.InternalLLMResponse) string {
	result := ""
	for _, choice := range stream.Choices {
		if choice.Delta != nil && choice.Delta.Content.Content != nil {
			result += *choice.Delta.Content.Content
		}
	}
	return result
}
