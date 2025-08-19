package main

import (
	"reflect"

	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func deriveSchemaFromValue(value any, reflector *jsonschema.Reflector) *jsonschema.Schema {
	v := reflect.ValueOf(value)
	t := reflect.TypeOf(value)

	// Dereference pointers
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}

	// Base switch by kind
	switch v.Kind() {
	case reflect.Struct:
		schema := &jsonschema.Schema{
			Type:       "object",
			Properties: orderedmap.New[string, *jsonschema.Schema](),
		}
		var required []string

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			jsonTag := field.Tag.Get("json")
			if jsonTag == "-" || jsonTag == "" {
				continue
			}
			name := jsonFieldName(jsonTag)
			fieldValue := v.Field(i)
			if fieldValue.IsZero() {
				continue
			}
			subSchema := deriveSchemaFromValue(fieldValue.Interface(), reflector)
			schema.Properties.Set(name, subSchema)
			required = append(required, name)
		}
		schema.Required = required
		return schema

	case reflect.Slice, reflect.Array:
		var itemSchemas []*jsonschema.Schema
		for i := 0; i < v.Len(); i++ {
			itemSchemas = append(itemSchemas, deriveSchemaFromValue(v.Index(i).Interface(), reflector))
		}
		numitems := uint64(v.Len())
		return &jsonschema.Schema{
			Type: "array",
			// Items: &jsonschema.Schema{
			// 	Schemas: jsonschema.Items(),
			// },
			AllOf:    itemSchemas,
			MinItems: &numitems,
			MaxItems: &numitems,
		}

	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			// Non-string keys aren't JSON-valid
			return &jsonschema.Schema{Type: "object"}
		}
		omap := orderedmap.New[string, *jsonschema.Schema]()
		schema := &jsonschema.Schema{
			Type:       "object",
			Properties: omap,
		}
		iter := v.MapRange()
		for iter.Next() {
			key := iter.Key().String()
			val := iter.Value().Interface()
			subSchema := deriveSchemaFromValue(val, reflector)
			schema.Properties.Set(key, subSchema)
		}
		return schema

	case reflect.String:
		return &jsonschema.Schema{Type: "string", Const: v.String()}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &jsonschema.Schema{Type: "integer", Const: v.Int()}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &jsonschema.Schema{Type: "integer", Const: v.Uint()}

	case reflect.Float32, reflect.Float64:
		return &jsonschema.Schema{Type: "number", Const: v.Float()}

	case reflect.Bool:
		return &jsonschema.Schema{Type: "boolean", Const: v.Bool()}

	default:
		return &jsonschema.Schema{Type: "null"}
	}
}

func jsonFieldName(tag string) string {
	if tag == "" {
		return ""
	}
	if tag == "-" {
		return ""
	}
	name := tag
	if idx := len(tag); idx >= 0 {
		name = tag
	}
	if comma := index(tag, ","); comma >= 0 {
		name = tag[:comma]
	}
	return name
}

func index(s, sep string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == sep[0] {
			return i
		}
	}
	return -1
}
