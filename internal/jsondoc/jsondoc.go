// Package jsondoc holds a JSON object as an ordered tree so that editing one
// key preserves every other key, its position and any value shape sugarlime
// does not understand.
package jsondoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Object is a JSON object that keeps its parsed key order.
// Values are *Object, []any, string, bool, json.Number or nil.
type Object struct {
	keys []string
	vals map[string]any
}

func New() *Object {
	return &Object{vals: map[string]any{}}
}

// Parse decodes a JSON object. An empty or whitespace-only input is an empty object.
func Parse(data []byte) (*Object, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return New(), nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("expected a JSON object at the top level, got %v", tok)
	}
	o, err := decodeObject(dec)
	if err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("unexpected trailing JSON")
	}
	return o, nil
}

func decodeObject(dec *json.Decoder) (*Object, error) {
	o := New()
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("expected an object key, got %v", tok)
		}
		val, err := decodeValue(dec)
		if err != nil {
			return nil, err
		}
		o.put(key, val)
	}
	if _, err := dec.Token(); err != nil { // consume '}'
		return nil, err
	}
	return o, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	switch d {
	case '{':
		return decodeObject(dec)
	case '[':
		arr := []any{}
		for dec.More() {
			v, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		if _, err := dec.Token(); err != nil { // consume ']'
			return nil, err
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("unexpected %v", d)
	}
}

func (o *Object) put(key string, val any) {
	if _, seen := o.vals[key]; !seen {
		o.keys = append(o.keys, key)
	}
	o.vals[key] = val
}

// Keys returns the object's own keys in order.
func (o *Object) Keys() []string {
	return append([]string(nil), o.keys...)
}

func (o *Object) Len() int { return len(o.keys) }

// Get resolves a dotted path such as "permissions.allow".
func (o *Object) Get(path string) (any, bool) {
	parts := splitPath(path)
	if len(parts) == 0 {
		return nil, false
	}
	cur := o
	for _, p := range parts[:len(parts)-1] {
		next, ok := cur.vals[p]
		if !ok {
			return nil, false
		}
		child, ok := next.(*Object)
		if !ok {
			return nil, false
		}
		cur = child
	}
	v, ok := cur.vals[parts[len(parts)-1]]
	return v, ok
}

func (o *Object) Has(path string) bool {
	_, ok := o.Get(path)
	return ok
}

// Set writes a value at a dotted path, creating intermediate objects.
func (o *Object) Set(path string, val any) error {
	parts := splitPath(path)
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}
	cur := o
	for i, p := range parts[:len(parts)-1] {
		existing, ok := cur.vals[p]
		if !ok {
			child := New()
			cur.put(p, child)
			cur = child
			continue
		}
		child, ok := existing.(*Object)
		if !ok {
			return fmt.Errorf("%s is not an object", strings.Join(parts[:i+1], "."))
		}
		cur = child
	}
	cur.put(parts[len(parts)-1], val)
	return nil
}

// Delete removes the key at path, and any now-empty parent objects it created.
func (o *Object) Delete(path string) bool {
	parts := splitPath(path)
	if len(parts) == 0 {
		return false
	}
	chain := []*Object{o}
	cur := o
	for _, p := range parts[:len(parts)-1] {
		child, ok := cur.vals[p].(*Object)
		if !ok {
			return false
		}
		cur = child
		chain = append(chain, child)
	}
	leaf := parts[len(parts)-1]
	if _, ok := cur.vals[leaf]; !ok {
		return false
	}
	cur.remove(leaf)
	for i := len(chain) - 1; i > 0; i-- {
		if chain[i].Len() > 0 {
			break
		}
		chain[i-1].remove(parts[i-1])
	}
	return true
}

func (o *Object) remove(key string) {
	delete(o.vals, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			return
		}
	}
}

func (o *Object) Clone() *Object {
	c := New()
	for _, k := range o.keys {
		c.put(k, cloneValue(o.vals[k]))
	}
	return c
}

func cloneValue(v any) any {
	switch t := v.(type) {
	case *Object:
		return t.Clone()
	case []any:
		arr := make([]any, len(t))
		for i, e := range t {
			arr[i] = cloneValue(e)
		}
		return arr
	default:
		return v
	}
}

// Encode renders the object as indented JSON with keys in their stored order.
func (o *Object) Encode() []byte {
	var buf bytes.Buffer
	writeValue(&buf, o, 0)
	buf.WriteByte('\n')
	return buf.Bytes()
}

func (o *Object) String() string { return string(o.Encode()) }

// Equal reports whether two objects render identically.
func Equal(a, b *Object) bool {
	return bytes.Equal(a.Encode(), b.Encode())
}

const indentUnit = "  "

func writeValue(buf *bytes.Buffer, v any, depth int) {
	switch t := v.(type) {
	case *Object:
		if t.Len() == 0 {
			buf.WriteString("{}")
			return
		}
		buf.WriteString("{\n")
		for i, k := range t.keys {
			writeIndent(buf, depth+1)
			writeString(buf, k)
			buf.WriteString(": ")
			writeValue(buf, t.vals[k], depth+1)
			if i < len(t.keys)-1 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		writeIndent(buf, depth)
		buf.WriteByte('}')
	case []any:
		if len(t) == 0 {
			buf.WriteString("[]")
			return
		}
		buf.WriteString("[\n")
		for i, e := range t {
			writeIndent(buf, depth+1)
			writeValue(buf, e, depth+1)
			if i < len(t)-1 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		writeIndent(buf, depth)
		buf.WriteByte(']')
	case string:
		writeString(buf, t)
	case json.Number:
		buf.WriteString(t.String())
	case float64:
		b, _ := json.Marshal(t)
		buf.Write(b)
	case int:
		fmt.Fprintf(buf, "%d", t)
	case bool:
		if t {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case nil:
		buf.WriteString("null")
	default:
		b, err := json.Marshal(t)
		if err != nil {
			buf.WriteString("null")
			return
		}
		buf.Write(b)
	}
}

func writeIndent(buf *bytes.Buffer, depth int) {
	for i := 0; i < depth; i++ {
		buf.WriteString(indentUnit)
	}
}

// writeString quotes a string without the HTML escaping encoding/json applies by default.
func writeString(buf *bytes.Buffer, s string) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		buf.WriteString(`""`)
		return
	}
	buf.Write(bytes.TrimRight(b.Bytes(), "\n"))
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}
