package configloader

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Load populates a struct from environment variables using struct tags.
// Tags:
//   - `env:"VAR_NAME"` — environment variable name
//   - `default:"value"` — default if env var is not set
//   - `required:"true"` — fail if no value and no default
//   - `description:"..."` — for documentation (not used at runtime)
//
// Supported types: string, int, int64, float64, bool, time.Duration, []string (comma-separated).
func Load(cfg interface{}) error {
	return loadWithPrefix(cfg, "")
}

// LoadWithPrefix loads config with an environment variable prefix.
// Example: prefix "APP" → looks for APP_PORT instead of PORT.
func LoadWithPrefix(cfg interface{}, prefix string) error {
	return loadWithPrefix(cfg, prefix)
}

func loadWithPrefix(cfg interface{}, prefix string) error {
	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("configloader: cfg must be a non-nil pointer to struct")
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("configloader: cfg must point to a struct")
	}

	return loadStruct(v, prefix)
}

func loadStruct(v reflect.Value, prefix string) error {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if !fv.CanSet() {
			continue
		}

		// Handle embedded structs.
		if field.Anonymous && fv.Kind() == reflect.Struct {
			if err := loadStruct(fv, prefix); err != nil {
				return err
			}
			continue
		}

		// Handle nested structs with prefix from field name.
		if fv.Kind() == reflect.Struct && fv.Type() != reflect.TypeOf(time.Duration(0)) {
			nestedPrefix := prefix
			if tag := field.Tag.Get("env_prefix"); tag != "" {
				if nestedPrefix != "" {
					nestedPrefix += "_"
				}
				nestedPrefix += tag
			}
			if err := loadStruct(fv, nestedPrefix); err != nil {
				return err
			}
			continue
		}

		envName := field.Tag.Get("env")
		if envName == "" {
			continue
		}

		if prefix != "" {
			envName = prefix + "_" + envName
		}

		defaultVal := field.Tag.Get("default")
		required := field.Tag.Get("required") == "true"

		val, found := os.LookupEnv(envName)
		if !found {
			if defaultVal != "" {
				val = defaultVal
			} else if required {
				return fmt.Errorf("configloader: required env var %s is not set", envName)
			} else {
				continue
			}
		}

		if err := setField(fv, val); err != nil {
			return fmt.Errorf("configloader: field %s (env %s): %w", field.Name, envName, err)
		}
	}

	return nil
}

func setField(fv reflect.Value, val string) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(val)

	case reflect.Int, reflect.Int64:
		// Check for time.Duration.
		if fv.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(val)
			if err != nil {
				return fmt.Errorf("invalid duration %q: %w", val, err)
			}
			fv.SetInt(int64(d))
			return nil
		}
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid int %q: %w", val, err)
		}
		fv.SetInt(n)

	case reflect.Float64:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return fmt.Errorf("invalid float %q: %w", val, err)
		}
		fv.SetFloat(f)

	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid bool %q: %w", val, err)
		}
		fv.SetBool(b)

	case reflect.Slice:
		if fv.Type().Elem().Kind() == reflect.String {
			parts := strings.Split(val, ",")
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			fv.Set(reflect.ValueOf(parts))
		} else {
			return fmt.Errorf("unsupported slice type: %s", fv.Type())
		}

	default:
		return fmt.Errorf("unsupported type: %s", fv.Kind())
	}

	return nil
}

// Validate runs validation on a loaded config struct.
// Fields with `validate:"nonempty"` must not be empty strings.
// Fields with `validate:"min=N"` must be >= N (int/float).
// Fields with `validate:"max=N"` must be <= N (int/float).
func Validate(cfg interface{}) error {
	v := reflect.ValueOf(cfg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return validateStruct(v)
}

func validateStruct(v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if fv.Kind() == reflect.Struct && fv.Type() != reflect.TypeOf(time.Duration(0)) {
			if err := validateStruct(fv); err != nil {
				return err
			}
			continue
		}

		tag := field.Tag.Get("validate")
		if tag == "" {
			continue
		}

		rules := strings.Split(tag, ",")
		for _, rule := range rules {
			if err := applyRule(field.Name, fv, strings.TrimSpace(rule)); err != nil {
				return err
			}
		}
	}
	return nil
}

func applyRule(name string, fv reflect.Value, rule string) error {
	switch {
	case rule == "nonempty":
		if fv.Kind() == reflect.String && fv.String() == "" {
			return fmt.Errorf("configloader: field %s must not be empty", name)
		}

	case strings.HasPrefix(rule, "min="):
		minStr := strings.TrimPrefix(rule, "min=")
		minVal, _ := strconv.ParseFloat(minStr, 64)
		switch fv.Kind() {
		case reflect.Int, reflect.Int64:
			if float64(fv.Int()) < minVal {
				return fmt.Errorf("configloader: field %s must be >= %s", name, minStr)
			}
		case reflect.Float64:
			if fv.Float() < minVal {
				return fmt.Errorf("configloader: field %s must be >= %s", name, minStr)
			}
		}

	case strings.HasPrefix(rule, "max="):
		maxStr := strings.TrimPrefix(rule, "max=")
		maxVal, _ := strconv.ParseFloat(maxStr, 64)
		switch fv.Kind() {
		case reflect.Int, reflect.Int64:
			if float64(fv.Int()) > maxVal {
				return fmt.Errorf("configloader: field %s must be <= %s", name, maxStr)
			}
		case reflect.Float64:
			if fv.Float() > maxVal {
				return fmt.Errorf("configloader: field %s must be <= %s", name, maxStr)
			}
		}
	}

	return nil
}

// Dump returns a map of env var names to their current values for logging.
// Sensitive fields (tagged `sensitive:"true"`) are masked.
func Dump(cfg interface{}) map[string]string {
	result := make(map[string]string)
	v := reflect.ValueOf(cfg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	dumpStruct(v, "", result)
	return result
}

func dumpStruct(v reflect.Value, prefix string, result map[string]string) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if fv.Kind() == reflect.Struct && fv.Type() != reflect.TypeOf(time.Duration(0)) {
			nestedPrefix := prefix
			if tag := field.Tag.Get("env_prefix"); tag != "" {
				if nestedPrefix != "" {
					nestedPrefix += "_"
				}
				nestedPrefix += tag
			}
			dumpStruct(fv, nestedPrefix, result)
			continue
		}

		envName := field.Tag.Get("env")
		if envName == "" {
			continue
		}
		if prefix != "" {
			envName = prefix + "_" + envName
		}

		if field.Tag.Get("sensitive") == "true" {
			result[envName] = "[REDACTED]"
		} else {
			result[envName] = fmt.Sprintf("%v", fv.Interface())
		}
	}
}
