package internal

import (
	"fmt"
	"strconv"
)

// getModuleName returns the "module" key from a step config map, defaulting to "payments".
func getModuleName(config map[string]any) string {
	if v, ok := config["module"].(string); ok && v != "" {
		return v
	}
	return "payments"
}

// resolveValue looks up key in current first, then config.
// Returns "" if not found.
func resolveValue(key string, current, config map[string]any) string {
	if v, ok := current[key].(string); ok && v != "" {
		return v
	}
	if v, ok := config[key].(string); ok && v != "" {
		return v
	}
	return ""
}

// resolveInt64 looks up key in current first, then config as int64.
func resolveInt64(key string, current, config map[string]any) int64 {
	if v := toInt64(current[key]); v != 0 {
		return v
	}
	return toInt64(config[key])
}

// resolveBool looks up key in current first, then config as bool.
func resolveBool(key string, current, config map[string]any) bool {
	if v, ok := toBool(current[key]); ok {
		return v
	}
	v, _ := toBool(config[key])
	return v
}

// resolveStringMap looks up key in current first, then config as map[string]string.
func resolveStringMap(key string, current, config map[string]any) map[string]string {
	if v := toStringMap(current[key]); len(v) > 0 {
		return v
	}
	return toStringMap(config[key])
}

// resolveFloat64 looks up key in current first, then config as float64.
func resolveFloat64(key string, current, config map[string]any) float64 {
	if v := toFloat64(current[key]); v != 0 {
		return v
	}
	return toFloat64(config[key])
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case int32:
		return int64(t)
	case float64:
		return int64(t)
	case float32:
		return int64(t)
	case string:
		var n int64
		fmt.Sscanf(t, "%d", &n)
		return n
	}
	return 0
}

func toBool(v any) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		b, err := strconv.ParseBool(t)
		return b, err == nil
	}
	return false, false
}

func toStringMap(v any) map[string]string {
	switch t := v.(type) {
	case map[string]string:
		return t
	case map[string]any:
		out := make(map[string]string, len(t))
		for k, v := range t {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
		return out
	}
	return nil
}

func toFloat64(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		var f float64
		fmt.Sscanf(t, "%f", &f)
		return f
	}
	return 0
}
