package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Edit changes one top-level key of settings.json. Raw is the JSON text to
// store, already formatted for the place it lands in.
type Edit struct {
	Key    string
	Raw    string
	Delete bool
}

var plainKey = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// Apply returns doc with the edits applied in order. Existing values are
// replaced in place and removed keys disappear with their separator, so the
// formatting and order of every other key stay byte for byte. A new key is
// appended in the document's own style (indent, line endings, compact or
// pretty). A nil or blank doc is treated as an empty object.
func Apply(doc []byte, edits []Edit) ([]byte, error) {
	if len(bytes.TrimSpace(doc)) == 0 {
		doc = []byte("{}\n")
	}
	if !gjson.ValidBytes(doc) {
		return nil, errors.New("document is not valid JSON")
	}
	if !gjson.ParseBytes(doc).IsObject() {
		return nil, errors.New("document is not a JSON object")
	}

	out := bytes.Clone(doc)
	for _, e := range edits {
		if !plainKey.MatchString(e.Key) {
			return nil, fmt.Errorf("unsupported settings key %q", e.Key)
		}
		exists := gjson.GetBytes(out, e.Key).Exists()
		var err error
		switch {
		case e.Delete && !exists:
		case e.Delete:
			out, err = sjson.DeleteBytes(out, e.Key)
		case !gjson.Valid(e.Raw):
			return nil, fmt.Errorf("value for %q is not valid JSON: %s", e.Key, e.Raw)
		case exists:
			out, err = sjson.SetRawBytes(out, e.Key, []byte(e.Raw))
		default:
			out = insertKey(out, e.Key, e.Raw)
		}
		if err != nil {
			return nil, fmt.Errorf("edit %q: %w", e.Key, err)
		}
	}
	return out, nil
}

// insertKey appends "key": raw as the last member of the root object, in the
// style of the existing document. doc must be a valid JSON object.
func insertKey(doc []byte, key, raw string) []byte {
	open := bytes.IndexByte(doc, '{')
	closeAt := bytes.LastIndexByte(doc, '}')
	multiline := bytes.Contains(doc, []byte("\n"))
	eol := "\n"
	if bytes.Contains(doc, []byte("\r\n")) {
		eol = "\r\n"
	}

	interior := doc[open+1 : closeAt]
	empty := len(bytes.TrimSpace(interior)) == 0

	indent := "  "
	if multiline && !empty {
		lead := interior[:len(interior)-len(bytes.TrimLeft(interior, " \t\r\n"))]
		if i := bytes.LastIndexByte(lead, '\n'); i >= 0 {
			indent = string(lead[i+1:])
		}
	}

	var member string
	if multiline {
		member = fmt.Sprintf("%q: %s", key, raw)
	} else {
		member = fmt.Sprintf("%q:%s", key, raw)
	}

	var out bytes.Buffer
	switch {
	case empty && multiline:
		out.Write(doc[:open+1])
		out.WriteString(eol + indent + member + eol)
		out.Write(doc[closeAt:])
	case empty:
		out.Write(doc[:open+1])
		out.WriteString(member)
		out.Write(doc[closeAt:])
	default:
		lastEnd := len(bytes.TrimRight(doc[:closeAt], " \t\r\n"))
		out.Write(doc[:lastEnd])
		if multiline {
			out.WriteString("," + eol + indent + member)
		} else {
			out.WriteString("," + member)
		}
		out.Write(doc[lastEnd:])
	}
	return out.Bytes()
}

// EqualJSON reports whether two JSON texts hold the same value, ignoring
// formatting and key order. Text that is not valid JSON is never equal.
func EqualJSON(a, b string) bool {
	var x, y any
	if json.Unmarshal([]byte(a), &x) != nil || json.Unmarshal([]byte(b), &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}
