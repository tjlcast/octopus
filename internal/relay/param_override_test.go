package relay

import (
	"reflect"
	"testing"
)

func TestMergeJSONObjectsPreservesNullAndDeletesExplicitPaths(t *testing.T) {
	payload := map[string]any{
		"model":       "gpt-4o",
		"temperature": float64(0.7),
		"metadata": map[string]any{
			"keep":   "yes",
			"remove": "no",
		},
	}
	overrides := map[string]any{
		"$delete": []any{"temperature", "metadata.remove"},
		"top_p":   nil,
		"metadata": map[string]any{
			"add": "new",
		},
	}

	mergeJSONObjects(payload, overrides)

	expected := map[string]any{
		"model": "gpt-4o",
		"top_p": nil,
		"metadata": map[string]any{
			"keep": "yes",
			"add":  "new",
		},
	}
	if !reflect.DeepEqual(payload, expected) {
		t.Fatalf("unexpected merged payload:\nwant: %#v\n got: %#v", expected, payload)
	}
}
