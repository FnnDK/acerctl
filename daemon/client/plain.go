package client

import "github.com/godbus/dbus/v5"

// Plain unwraps D-Bus variants (and the nested maps the daemon uses) into
// plain Go values, ready for JSON or simple type switches.
func Plain(v interface{}) interface{} {
	switch t := v.(type) {
	case dbus.Variant:
		return Plain(t.Value())
	case map[string]dbus.Variant:
		m := map[string]interface{}{}
		for k, x := range t {
			m[k] = Plain(x)
		}
		return m
	case map[string]map[string]dbus.Variant:
		m := map[string]interface{}{}
		for k, x := range t {
			m[k] = Plain(x)
		}
		return m
	case map[string]map[string]interface{}:
		return t
	case map[string]interface{}:
		m := map[string]interface{}{}
		for k, x := range t {
			m[k] = Plain(x)
		}
		return m
	case []map[string]dbus.Variant:
		out := make([]interface{}, len(t))
		for i, x := range t {
			out[i] = Plain(x)
		}
		return out
	}
	return v
}

// PlainMap is Plain for values known to be dictionaries; never nil.
func PlainMap(v interface{}) map[string]interface{} {
	m, _ := Plain(v).(map[string]interface{})
	if m == nil {
		m = map[string]interface{}{}
	}
	return m
}
