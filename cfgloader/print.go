package cfgloader

import (
	"log/slog"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

func printConfig(config any) {
	masked := maskStruct(config)

	out, err := yaml.Marshal(masked)
	if err != nil {
		slog.Error("failed to marshal config", "error", err.Error())
	}
	slog.Info(string(out))
}

func maskStruct(cfg any) any {
	val := reflect.ValueOf(cfg)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	return maskValue(val).Interface()
}

func maskValue(val reflect.Value) reflect.Value { //nolint:gocognit // not complex enough to split
	if !val.IsValid() {
		return val
	}

	switch val.Kind() { //nolint:exhaustive // only handled kinds relevant to masking
	case reflect.Ptr:
		if val.IsNil() {
			return val
		}
		ptr := reflect.New(val.Elem().Type())
		ptr.Elem().Set(maskValue(val.Elem()))
		return ptr

	case reflect.Struct:
		masked := reflect.New(val.Type()).Elem()
		numFields := val.NumField()
		for i := range numFields {
			field := val.Type().Field(i)
			origVal := val.Field(i)

			if !masked.Field(i).CanSet() || !origVal.CanInterface() {
				continue
			}

			if field.Tag.Get("mask") == "true" {
				masked.Field(i).Set(maskAny(origVal))
			} else {
				masked.Field(i).Set(maskValue(origVal))
			}
		}
		return masked

	case reflect.Slice:
		masked := reflect.MakeSlice(val.Type(), val.Len(), val.Cap())
		length := val.Len()
		for i := range length {
			masked.Index(i).Set(maskValue(val.Index(i)))
		}
		return masked

	case reflect.Array:
		masked := reflect.New(val.Type()).Elem()
		length := val.Len()
		for i := range length {
			masked.Index(i).Set(maskValue(val.Index(i)))
		}
		return masked

	case reflect.Map:
		masked := reflect.MakeMapWithSize(val.Type(), val.Len())
		for _, key := range val.MapKeys() {
			masked.SetMapIndex(key, maskValue(val.MapIndex(key)))
		}
		return masked

	case reflect.Interface:
		if val.IsNil() {
			return val
		}
		return maskValue(val.Elem())

	case reflect.String:
		return reflect.ValueOf(maskString(val.String()))

	default:
		return val
	}
}

func maskAny(val reflect.Value) reflect.Value {
	if !val.IsValid() {
		return val
	}

	switch val.Kind() { //nolint: exhaustive // only handled kinds relevant to masking
	case reflect.String:
		return reflect.ValueOf(maskString(val.String()))

	case reflect.Struct, reflect.Slice, reflect.Array, reflect.Map, reflect.Interface, reflect.Ptr:
		return maskValue(val)

	default:
		return reflect.Zero(val.Type())
	}
}

func maskString(s string) string {
	return strings.Repeat("*", len(s))
}
