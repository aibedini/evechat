package services

import (
	"strconv"
	"strings"
)

// step is one hop of a compiled JSONPath.
type step struct {
	index int
	key   string
	isIdx bool
}

// jsonPathGet resolves the small JSONPath subset the channel mapping needs:
//
//	$.user.id        nested key
//	$.items[0].name  array index
//	$['a b']         quoted key
//
// Wildcards, filters and recursive descent are deliberately unsupported: a
// mapping is configuration, and a configuration language that can run away is
// a liability.
func jsonPathGet(root any, path string) (any, bool) {
	steps, ok := compilePath(path)
	if !ok {
		return nil, false
	}
	cur := root
	for _, s := range steps {
		if s.isIdx {
			arr, isArr := cur.([]any)
			if !isArr || s.index < 0 || s.index >= len(arr) {
				return nil, false
			}
			cur = arr[s.index]
			continue
		}
		m, isMap := cur.(map[string]any)
		if !isMap {
			return nil, false
		}
		v, exists := m[s.key]
		if !exists {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

func compilePath(path string) ([]step, bool) {
	path = strings.TrimSpace(path)
	if path == "$" || path == "$." {
		return nil, true
	}
	if !strings.HasPrefix(path, "$") {
		return nil, false
	}
	rest := strings.TrimPrefix(path, "$")
	if strings.HasPrefix(rest, ".") {
		rest = rest[1:]
	}
	if rest == "" {
		return nil, true
	}
	segments := splitSegments(rest)
	steps := make([]step, 0, len(segments)+2)
	for _, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			return nil, false
		}
		parsed, ok := parseSegment(seg)
		if !ok {
			return nil, false
		}
		steps = append(steps, parsed...)
	}
	return steps, len(steps) > 0
}

// parseSegment turns `items[1]`, `[0]` or `['a b']` into steps.
func parseSegment(seg string) ([]step, bool) {
	out := make([]step, 0, 2)
	for len(seg) > 0 {
		if seg[0] == '[' {
			end := strings.IndexByte(seg, ']')
			if end < 0 {
				return nil, false
			}
			inner := strings.TrimSpace(seg[1:end])
			if inner == "" || strings.Contains(inner, "*") {
				return nil, false
			}
			if inner[0] == '\'' || inner[0] == '"' {
				if len(inner) < 2 {
					return nil, false
				}
				out = append(out, step{key: inner[1 : len(inner)-1]})
			} else {
				n, err := strconv.Atoi(inner)
				if err != nil {
					return nil, false
				}
				out = append(out, step{index: n, isIdx: true})
			}
			seg = seg[end+1:]
			continue
		}
		if seg[0] == '.' {
			return nil, false
		}
		keyEnd := strings.IndexAny(seg, "[.")
		if keyEnd < 0 {
			keyEnd = len(seg)
		}
		key := seg[:keyEnd]
		if key == "" || strings.ContainsAny(key, "*]") {
			return nil, false
		}
		out = append(out, step{key: key})
		seg = seg[keyEnd:]
	}
	return out, len(out) > 0
}

// splitSegments splits on dots that are not inside brackets or quotes.
func splitSegments(path string) []string {
	var (
		out    []string
		buf    strings.Builder
		depth  int
		quoted bool
	)
	for _, r := range path {
		switch {
		case r == '\'' || r == '"':
			quoted = !quoted
			buf.WriteRune(r)
		case r == '[' && !quoted:
			depth++
			buf.WriteRune(r)
		case r == ']' && !quoted:
			if depth > 0 {
				depth--
			}
			buf.WriteRune(r)
		case r == '.' && depth == 0 && !quoted:
			out = append(out, buf.String())
			buf.Reset()
		default:
			buf.WriteRune(r)
		}
	}
	out = append(out, buf.String())
	return out
}

// jsonPathString resolves a path and renders the result as a string.
func jsonPathString(root any, path string) string {
	v, ok := jsonPathGet(root, path)
	if !ok || v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}
