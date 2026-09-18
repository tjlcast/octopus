package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const deleteOverrideKey = "$delete"

func (ra *relayAttempt) applyParamOverride(req *http.Request) error {
	if ra.channel.ParamOverride == nil || strings.TrimSpace(*ra.channel.ParamOverride) == "" {
		return nil
	}
	if req == nil || req.Body == nil {
		return nil
	}

	var overrides map[string]any
	if err := json.Unmarshal([]byte(*ra.channel.ParamOverride), &overrides); err != nil {
		return fmt.Errorf("invalid param override json: %w", err)
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	_ = req.Body.Close()

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		return fmt.Errorf("request body is not a json object: %w", err)
	}

	mergeJSONObjects(payload, overrides)

	merged, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal overridden request body: %w", err)
	}

	req.Body = io.NopCloser(bytes.NewReader(merged))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(merged)), nil
	}
	req.ContentLength = int64(len(merged))
	return nil
}

func mergeJSONObjects(dst, src map[string]any) {
	for key, value := range src {
		if key == deleteOverrideKey {
			deleteJSONPaths(dst, value)
			continue
		}

		srcObj, srcIsObj := value.(map[string]any)
		dstObj, dstIsObj := dst[key].(map[string]any)
		if srcIsObj && dstIsObj {
			mergeJSONObjects(dstObj, srcObj)
			continue
		}
		dst[key] = value
	}
}

func deleteJSONPaths(dst map[string]any, paths any) {
	for _, path := range stringSlice(paths) {
		deleteJSONPath(dst, path)
	}
}

func deleteJSONPath(dst map[string]any, path string) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return
	}

	current := dst
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			return
		}
		current = next
	}
	delete(current, parts[len(parts)-1])
}

func stringSlice(value any) []string {
	switch v := value.(type) {
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			path, ok := item.(string)
			if ok && strings.TrimSpace(path) != "" {
				result = append(result, path)
			}
		}
		return result
	case []string:
		return v
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{v}
	default:
		return nil
	}
}
