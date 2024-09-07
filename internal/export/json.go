package export

import (
	"fmt"
	"reflect"
	"strings"
)

type Optional[T any] struct {
	Value T
	Set   bool
}

func (o Optional[T]) ValOk() (interface{}, bool) {
	return o.Value, o.Set
}

func Some[T any](val T) Optional[T] {
	return Optional[T]{
		Value: val,
		Set:   true,
	}
}

func Maybe[T any](val *T) Optional[T] {
	if val == nil {
		return Optional[T]{
			Set: false,
		}
	}
	return Optional[T]{
		Value: *val,
		Set:   true,
	}
}

func Value[T any](val T) Optional[T] {
	return Optional[T]{
		Value: val,
		Set:   true,
	}
}

func Ptr[T any](val T) *T {
	return &val
}

func jsonStructFields(object interface{}, out map[string]interface{}) error {
	if out == nil {
		return fmt.Errorf("jsonFieldMap requires a non-nil map")
	}

	val := reflect.ValueOf(object)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return fmt.Errorf("jsonFieldMap requires a non-nil struct")
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("jsonFieldMap requires a struct, got %s", val.Kind().String())
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Type().Field(i)
		if field.Anonymous {
			err := jsonStructFields(val.Field(i).Interface(), out)
			if err != nil {
				return fmt.Errorf("anon field %s: %w", field.Name, err)
			}
			continue
		}

		tag := field.Tag.Get("json")
		if tag == "" {

			// maybe map lower case?
			continue
		}
		parts := strings.Split(tag, ",")
		name := parts[0]
		if name == "-" {
			continue
		}
		omitempty := false
		for _, part := range parts[1:] {
			if part == "omitempty" {
				omitempty = true
			}
		}

		if omitempty && val.Field(i).IsZero() {
			continue
		}
		iv := val.Field(i).Interface()
		if iv == nil {
			continue
		}
		if optional, ok := iv.(interface{ ValOk() (interface{}, bool) }); ok {
			val, isSet := optional.ValOk()
			if !isSet {
				continue
			}
			iv = val
		}

		out[name] = iv
	}

	return nil
}
