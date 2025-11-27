// Package mask provides functionality for masking sensitive fields in structs before logging or other debugging tasks.
package mask

import (
	"fmt"
	"reflect"
	"strings"

	orderedmap "github.com/wk8/go-ordered-map/v2"
)

const tagName = "mask"

// StructToOrdMap returns an ordered map of fields with sensitive values masked.
// Fields tagged with `mask:"true"` will have their values replaced.
// Field names are determined by priority: json tag > yaml tag > struct field name.
// Fields with json:"-" or yaml:"-" are excluded from the output.
func StructToOrdMap(v any) *orderedmap.OrderedMap[string, any] {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	return maskToOrdMap(val, "")
}

func maskToOrdMap(val reflect.Value, prefix string) *orderedmap.OrderedMap[string, any] {
	// Dereference pointer
	if val.Kind() == reflect.Pointer {
		if val.IsNil() {
			om := orderedmap.New[string, any]()
			om.Set(prefix, nil)
			return om
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		om := orderedmap.New[string, any]()
		om.Set(prefix, val.Interface())
		return om
	}

	om := orderedmap.New[string, any]()
	typ := val.Type()

	for i := range val.NumField() {
		field := val.Field(i)
		fieldType := typ.Field(i)

		if !fieldType.IsExported() {
			continue
		}

		// Extract field name from tags
		fieldName, shouldSkip := extractFieldName(fieldType)
		if shouldSkip {
			continue // Skip fields with json:"-" or yaml:"-"
		}

		name := fieldName
		if prefix != "" {
			name = prefix + "." + name
		}

		switch {
		case shouldMask(fieldType):
			om.Set(name, maskValue(field))
		case isExpandable(field):
			nested := maskToOrdMap(field, name)
			for pair := nested.Oldest(); pair != nil; pair = pair.Next() {
				om.Set(pair.Key, pair.Value)
			}
		default:
			om.Set(name, field.Interface())
		}
	}

	return om
}

func isExpandable(val reflect.Value) bool {
	kind := val.Kind()
	if kind == reflect.Pointer {
		if val.IsNil() {
			return false
		}
		kind = val.Elem().Kind()
	}
	return kind == reflect.Struct
}

func shouldMask(field reflect.StructField) bool {
	tag := field.Tag.Get(tagName)
	return strings.EqualFold(tag, "true")
}

func maskValue(val reflect.Value) any {
	// Check for nil pointers, slices, and maps
	switch val.Kind() { //nolint:exhaustive // Default case handles remaining types
	case reflect.Pointer:
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	case reflect.Slice, reflect.Map:
		if val.IsNil() {
			return nil
		}
	}

	// Don't mask zero values of any type
	if val.IsZero() {
		return val.Interface()
	}

	return maskByKind(val)
}

func maskByKind(val reflect.Value) any {
	//nolint:exhaustive // Default case handles remaining types
	switch val.Kind() {
	case reflect.String:
		return "***masked-string***"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "***masked-int***"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "***masked-uint***"
	case reflect.Float32, reflect.Float64:
		return "***masked-float***"
	case reflect.Bool:
		return "***masked-bool***"
	case reflect.Struct:
		return "***masked-struct***"
	case reflect.Slice, reflect.Array:
		return "***masked-slice***"
	case reflect.Map:
		return "***masked-map***"
	default:
		return fmt.Sprintf("***masked-%s***", val.Kind())
	}
}

// extractFieldName extracts the field name from struct tags with priority:
// 1. json tag (if present and not "-")
// 2. yaml tag (if present and not "-")
// 3. struct field name
// Returns (fieldName, shouldSkip) where shouldSkip=true means field should be omitted.
func extractFieldName(field reflect.StructField) (string, bool) {
	// Check json tag first
	if jsonTag, ok := field.Tag.Lookup("json"); ok {
		if jsonTag == "-" {
			return "", true
		}
		// Extract name before comma (e.g., "fieldname,omitempty" -> "fieldname")
		if idx := strings.Index(jsonTag, ","); idx != -1 {
			jsonTag = jsonTag[:idx]
		}
		if jsonTag != "" {
			return jsonTag, false
		}
	}

	// Check yaml tag second
	if yamlTag, ok := field.Tag.Lookup("yaml"); ok {
		if yamlTag == "-" {
			return "", true
		}
		if idx := strings.Index(yamlTag, ","); idx != -1 {
			yamlTag = yamlTag[:idx]
		}
		if yamlTag != "" {
			return yamlTag, false
		}
	}

	// Fallback to struct field name
	return field.Name, false
}
