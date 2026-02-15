package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Validate checks struct fields against their `validate` tags.
// Supported tags:
//
//	validate:"required"          - field must not be zero value
//	validate:"min=1"             - int/float minimum
//	validate:"max=65535"         - int/float maximum
//	validate:"oneof=a b c"      - string must be one of the listed values
//
// Tags can be combined: validate:"required,min=1,max=100"
func Validate(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("validate: expected struct, got %s", rv.Kind())
	}
	return validateStruct(rv)
}

func validateStruct(rv reflect.Value) error {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fv := rv.Field(i)

		tag := field.Tag.Get("validate")
		if tag == "" || tag == "-" {
			continue
		}

		// Recurse into nested structs.
		if fv.Kind() == reflect.Struct && tag == "" {
			if err := validateStruct(fv); err != nil {
				return fmt.Errorf("%s.%w", field.Name, err)
			}
			continue
		}

		if err := validateField(field.Name, fv, tag); err != nil {
			return err
		}
	}
	return nil
}

func validateField(name string, fv reflect.Value, tag string) error {
	rules := strings.Split(tag, ",")
	for _, rule := range rules {
		rule = strings.TrimSpace(rule)

		switch {
		case rule == "required":
			if fv.IsZero() {
				return fmt.Errorf("config: %s is required", name)
			}

		case strings.HasPrefix(rule, "min="):
			minStr := rule[4:]
			if err := checkMin(name, fv, minStr); err != nil {
				return err
			}

		case strings.HasPrefix(rule, "max="):
			maxStr := rule[4:]
			if err := checkMax(name, fv, maxStr); err != nil {
				return err
			}

		case strings.HasPrefix(rule, "oneof="):
			allowed := strings.Fields(rule[6:])
			if err := checkOneOf(name, fv, allowed); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkMin(name string, fv reflect.Value, minStr string) error {
	switch fv.Kind() {
	case reflect.Int, reflect.Int64:
		min, err := strconv.ParseInt(minStr, 10, 64)
		if err != nil {
			return fmt.Errorf("config: invalid min for %s: %s", name, minStr)
		}
		if fv.Int() < min {
			return fmt.Errorf("config: %s must be >= %d (got %d)", name, min, fv.Int())
		}
	case reflect.Float64:
		min, err := strconv.ParseFloat(minStr, 64)
		if err != nil {
			return fmt.Errorf("config: invalid min for %s: %s", name, minStr)
		}
		if fv.Float() < min {
			return fmt.Errorf("config: %s must be >= %g (got %g)", name, min, fv.Float())
		}
	case reflect.String:
		min, err := strconv.Atoi(minStr)
		if err != nil {
			return fmt.Errorf("config: invalid min length for %s: %s", name, minStr)
		}
		if len(fv.String()) < min {
			return fmt.Errorf("config: %s must be at least %d characters (got %d)", name, min, len(fv.String()))
		}
	}
	return nil
}

func checkMax(name string, fv reflect.Value, maxStr string) error {
	switch fv.Kind() {
	case reflect.Int, reflect.Int64:
		max, err := strconv.ParseInt(maxStr, 10, 64)
		if err != nil {
			return fmt.Errorf("config: invalid max for %s: %s", name, maxStr)
		}
		if fv.Int() > max {
			return fmt.Errorf("config: %s must be <= %d (got %d)", name, max, fv.Int())
		}
	case reflect.Float64:
		max, err := strconv.ParseFloat(maxStr, 64)
		if err != nil {
			return fmt.Errorf("config: invalid max for %s: %s", name, maxStr)
		}
		if fv.Float() > max {
			return fmt.Errorf("config: %s must be <= %g (got %g)", name, max, fv.Float())
		}
	case reflect.String:
		max, err := strconv.Atoi(maxStr)
		if err != nil {
			return fmt.Errorf("config: invalid max length for %s: %s", name, maxStr)
		}
		if len(fv.String()) > max {
			return fmt.Errorf("config: %s must be at most %d characters (got %d)", name, max, len(fv.String()))
		}
	}
	return nil
}

func checkOneOf(name string, fv reflect.Value, allowed []string) error {
	if fv.Kind() != reflect.String {
		return fmt.Errorf("config: oneof only works with strings, %s is %s", name, fv.Kind())
	}
	val := fv.String()
	for _, a := range allowed {
		if val == a {
			return nil
		}
	}
	return fmt.Errorf("config: %s must be one of [%s] (got %q)", name, strings.Join(allowed, ", "), val)
}
