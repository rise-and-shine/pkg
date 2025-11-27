// Package mask provides functionality for masking sensitive fields in structs before logging.
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
func StructToOrdMap(v any) *orderedmap.OrderedMap[string, any] {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	return maskToOrdMap(val, "")
}

func maskToOrdMap(val reflect.Value, prefix string) *orderedmap.OrderedMap[string, any] {
	// Dereference pointer
	if val.Kind() == reflect.Ptr {
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

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		if !fieldType.IsExported() {
			continue
		}

		name := fieldType.Name
		if prefix != "" {
			name = prefix + "." + name
		}

		if shouldMask(fieldType) {
			om.Set(name, maskValue(field))
		} else if isExpandable(field) {
			nested := maskToOrdMap(field, name)
			for pair := nested.Oldest(); pair != nil; pair = pair.Next() {
				om.Set(pair.Key, pair.Value)
			}
		} else {
			om.Set(name, field.Interface())
		}
	}

	return om
}

func isExpandable(val reflect.Value) bool {
	kind := val.Kind()
	if kind == reflect.Ptr {
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
	switch val.Kind() {
	case reflect.Ptr:
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	case reflect.Slice, reflect.Map:
		if val.IsNil() {
			return nil
		}
	}

	return maskByKind(val)
}

func maskByKind(val reflect.Value) any {
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
