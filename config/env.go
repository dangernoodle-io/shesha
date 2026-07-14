package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"

	"github.com/dangernoodle-io/shesha/internal/keyname"
)

// WithEnv overlays environment variables onto the current value: for each
// exported scalar field, it looks up prefix+keyname.Upper(fieldName) and, if
// set, parses it into the field. Non-scalar fields (struct, slice, map,
// pointer, interface) are silently skipped. There is no aliasing — a field
// is only ever read from its single derived name; consumers that need
// aliases handle that themselves.
func WithEnv(prefix string) Option {
	return func(p *plan) {
		p.envPrefix = prefix
		p.hasEnv = true
	}
}

// overlayEnv reflects over out (a *T) and applies the prefix+field-name env
// var overlay described by WithEnv. out must point to a struct; any other
// kind is a Load-time error rather than a panic.
func overlayEnv[T any](prefix string, out *T) error {
	v := reflect.ValueOf(out).Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("config: WithEnv requires a struct type, got %s", v.Kind())
	}

	t := v.Type()

	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		fv := v.Field(i)
		if !isScalarKind(fv.Kind()) {
			continue
		}

		envName := prefix + keyname.Upper(field.Name)

		raw, ok := os.LookupEnv(envName)
		if !ok {
			continue
		}

		if err := setScalar(fv, field.Name, envName, raw); err != nil {
			return err
		}
	}

	return nil
}

// isScalarKind reports whether kind is one WithEnv knows how to set from a
// string env value.
func isScalarKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// setScalar parses raw and sets it into fv per fv's Kind. fieldName and
// envName are carried only for the error message on parse failure.
func setScalar(fv reflect.Value, fieldName, envName, raw string) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("config: field %s (env %s): invalid value %q: %w", fieldName, envName, raw, err)
		}

		fv.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, fv.Type().Bits())
		if err != nil {
			return fmt.Errorf("config: field %s (env %s): invalid value %q: %w", fieldName, envName, raw, err)
		}

		fv.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, fv.Type().Bits())
		if err != nil {
			return fmt.Errorf("config: field %s (env %s): invalid value %q: %w", fieldName, envName, raw, err)
		}

		fv.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(raw, fv.Type().Bits())
		if err != nil {
			return fmt.Errorf("config: field %s (env %s): invalid value %q: %w", fieldName, envName, raw, err)
		}

		fv.SetFloat(n)
	}

	return nil
}
