package logforwarder

import (
	"time"
)

func flattenStructured(sd *map[string]map[string]string) map[string]string {
	if sd == nil || len(*sd) == 0 {
		return nil
	}

	// Note:
	// This uses "blockID.key" flattening for simplicity.
	// If block IDs or keys can contain dots and you need collision-free encoding,
	// switch to a nested structure or an escaping scheme.
	out := make(map[string]string, 8)
	for blockID, params := range *sd {
		for k, v := range params {
			out[blockID+"."+k] = v
		}
	}
	return out
}

func timestampString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func deref[T ~string](p *T) string {
	if p == nil {
		return ""
	}
	return string(*p)
}
