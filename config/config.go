package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// Load reads a TOML file and decodes it into the struct pointed to by v.
// Environment variables with the given prefix override file values.
// For a prefix "MATCHSPEC" and a field "Port", MATCHSPEC_PORT wins.
func Load(path, envPrefix string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	defer f.Close()

	data, err := ParseTOML(f)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if err := Decode(data, v); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	if envPrefix != "" {
		applyEnv(envPrefix, v)
	}

	return nil
}

// Decode maps a parsed TOML map onto a struct. Fields are matched by
// their toml tag, or by lowercased field name if no tag is present.
func Decode(data map[string]any, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("decode: expected pointer to struct")
	}
	return decodeStruct(data, rv.Elem())
}

func decodeStruct(data map[string]any, rv reflect.Value) error {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fv := rv.Field(i)
		if !fv.CanSet() {
			continue
		}

		key := field.Tag.Get("toml")
		if key == "" {
			key = strings.ToLower(field.Name)
		}

		val, ok := data[key]
		if !ok {
			continue
		}

		if err := setField(fv, val); err != nil {
			return fmt.Errorf("field %s: %w", field.Name, err)
		}
	}
	return nil
}

func setField(fv reflect.Value, val any) error {
	switch fv.Kind() {
	case reflect.String:
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", val)
		}
		fv.SetString(s)

	case reflect.Int, reflect.Int64:
		switch v := val.(type) {
		case int64:
			fv.SetInt(v)
		case float64:
			fv.SetInt(int64(v))
		default:
			return fmt.Errorf("expected int, got %T", val)
		}

	case reflect.Float64:
		switch v := val.(type) {
		case float64:
			fv.SetFloat(v)
		case int64:
			fv.SetFloat(float64(v))
		default:
			return fmt.Errorf("expected float, got %T", val)
		}

	case reflect.Bool:
		b, ok := val.(bool)
		if !ok {
			return fmt.Errorf("expected bool, got %T", val)
		}
		fv.SetBool(b)

	case reflect.Slice:
		arr, ok := val.([]any)
		if !ok {
			return fmt.Errorf("expected array, got %T", val)
		}
		slice := reflect.MakeSlice(fv.Type(), len(arr), len(arr))
		for i, elem := range arr {
			if err := setField(slice.Index(i), elem); err != nil {
				return fmt.Errorf("index %d: %w", i, err)
			}
		}
		fv.Set(slice)

	case reflect.Struct:
		m, ok := val.(map[string]any)
		if !ok {
			return fmt.Errorf("expected table, got %T", val)
		}
		return decodeStruct(m, fv)

	default:
		return fmt.Errorf("unsupported kind: %s", fv.Kind())
	}

	return nil
}

func applyEnv(prefix string, v any) {
	rv := reflect.ValueOf(v).Elem()
	rt := rv.Type()
	prefix = strings.ToUpper(prefix)

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fv := rv.Field(i)
		if !fv.CanSet() {
			continue
		}

		envKey := prefix + "_" + strings.ToUpper(field.Name)
		envVal, ok := os.LookupEnv(envKey)
		if !ok {
			continue
		}

		switch fv.Kind() {
		case reflect.String:
			fv.SetString(envVal)
		case reflect.Int, reflect.Int64:
			if n, err := strconv.ParseInt(envVal, 10, 64); err == nil {
				fv.SetInt(n)
			}
		case reflect.Float64:
			if f, err := strconv.ParseFloat(envVal, 64); err == nil {
				fv.SetFloat(f)
			}
		case reflect.Bool:
			fv.SetBool(envVal == "true" || envVal == "1")
		}
	}
}
