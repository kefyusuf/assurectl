package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"
)

var rawMessageType = reflect.TypeOf(json.RawMessage{})

func Decode(data []byte, dst any) error {
	if !utf8.Valid(data) {
		return errors.New("invalid JSON: input is not valid UTF-8")
	}
	if err := validateStructure(data); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := validateExactFieldNames(data, reflect.TypeOf(dst)); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("decode JSON: multiple JSON values")
		}
		return fmt.Errorf("decode JSON: %w", err)
	}

	return nil
}

func validateStructure(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanValue(decoder); err != nil {
		return err
	}

	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func scanValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}

	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate object key %q", key)
			}
			seen[key] = struct{}{}
			if err := scanValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return errors.New("object is not properly terminated")
		}
	case '[':
		for decoder.More() {
			if err := scanValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return errors.New("array is not properly terminated")
		}
	default:
		return fmt.Errorf("unexpected delimiter %q", delim)
	}

	return nil
}

func validateExactFieldNames(data []byte, dstType reflect.Type) error {
	if dstType == nil {
		return errors.New("destination type is nil")
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}

	return validateValueShape(value, dstType)
}

func validateValueShape(value any, typ reflect.Type) error {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == rawMessageType {
		return nil
	}

	switch typ.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}

		fields := jsonFields(typ)
		for key, child := range object {
			fieldType, exists := fields[key]
			if !exists {
				return fmt.Errorf("unknown field %q", key)
			}
			if err := validateValueShape(child, fieldType); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		array, ok := value.([]any)
		if !ok {
			return nil
		}
		for _, child := range array {
			if err := validateValueShape(child, typ.Elem()); err != nil {
				return err
			}
		}
	case reflect.Map:
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		for _, child := range object {
			if err := validateValueShape(child, typ.Elem()); err != nil {
				return err
			}
		}
	}

	return nil
}

func jsonFields(typ reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type)
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}

		tag := field.Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		fields[name] = field.Type
	}
	return fields
}
