package jsondoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Str returns the string at path.
func (o *Object) Str(path string) (string, bool) {
	v, ok := o.Get(path)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// Bool returns the boolean at path.
func (o *Object) Bool(path string) (bool, bool) {
	v, ok := o.Get(path)
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// Num returns the number at path.
func (o *Object) Num(path string) (json.Number, bool) {
	v, ok := o.Get(path)
	if !ok {
		return "", false
	}
	n, ok := v.(json.Number)
	return n, ok
}

// Strings returns the array at path when every element is a string.
func (o *Object) Strings(path string) ([]string, bool) {
	v, ok := o.Get(path)
	if !ok {
		return nil, false
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		s, ok := e.(string)
		if !ok {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// SetStrings writes a string array, deleting the key when the list is empty.
func (o *Object) SetStrings(path string, vals []string) error {
	if len(vals) == 0 {
		o.Delete(path)
		return nil
	}
	arr := make([]any, len(vals))
	for i, v := range vals {
		arr[i] = v
	}
	return o.Set(path, arr)
}

// SetRaw parses a JSON fragment and writes it at path.
func (o *Object) SetRaw(path, fragment string) error {
	dec := json.NewDecoder(strings.NewReader(fragment))
	dec.UseNumber()
	v, err := decodeValue(dec)
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if dec.More() {
		return fmt.Errorf("invalid JSON: unexpected trailing content")
	}
	return o.Set(path, v)
}

// Render formats a value as the JSON text an editor should show.
func Render(v any) string {
	var buf bytes.Buffer
	writeValue(&buf, v, 0)
	return buf.String()
}

// Summary describes a value in one line, for a list column.
func Summary(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		if t {
			return "true"
		}
		return "false"
	case string:
		return t
	case json.Number:
		return t.String()
	case []any:
		return pluralize(len(t), "item")
	case *Object:
		return pluralize(t.Len(), "key")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func pluralize(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
