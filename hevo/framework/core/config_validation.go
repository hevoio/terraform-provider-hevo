package core

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

// ValidateConfig checks config against a connector's schema: every key is one the connector
// knows, values have the right type, allowed value and pattern, and required keys are present,
// including keys required under a choice (if/then), as Hevo checks them with defaults filled in.
func ValidateConfig(config map[string]any, schema *Property, root path.Path, diags *diag.Diagnostics) {
	if schema == nil {
		return
	}

	validateObject(config, schema, root, diags)
}

// validateConditions checks the keys an object requires under its if/then rules. A key not set
// counts as its schema default, as Hevo fills it in. The message is Hevo's own.
func validateConditions(values map[string]any, schema *Property, at path.Path, diags *diag.Diagnostics) {
	var missing []string

	for _, c := range schema.Conditions {
		if !conditionHolds(values, schema, c) {
			continue
		}

		for _, key := range c.Required {
			if value, ok := values[key]; (!ok || value == nil) && !slices.Contains(missing, key) {
				missing = append(missing, key)
			}
		}
	}

	if len(missing) == 0 {
		return
	}

	sort.Strings(missing)
	diags.AddAttributeError(at, "Missing required config key",
		fmt.Sprintf("The configuration is incomplete because no value was specified for the following fields: %s. "+
			"Specify a value for each of them.", strings.Join(missing, ", ")))
}

func conditionHolds(values map[string]any, schema *Property, c Condition) bool {
	for key, want := range c.When {
		value, ok := values[key]
		if !ok || value == nil {
			prop := schema.property(key)
			if prop == nil || prop.Default == nil {
				return false
			}

			value = prop.Default
		}

		if !ScalarsEqual(value, want) {
			return false
		}
	}

	return true
}

func validateObject(values map[string]any, schema *Property, at path.Path, diags *diag.Diagnostics) {
	if len(schema.Properties) > 0 {
		for _, key := range sortedKeys(values) {
			if _, ok := schema.Properties[key]; !ok {
				diags.AddAttributeError(at.AtMapKey(key), "Unknown config key",
					fmt.Sprintf("The connector has no setting %q. Known settings: %s.", key, strings.Join(sortedKeys(schema.Properties), ", ")))
			}
		}
	}

	for _, key := range schema.Required {
		if value, ok := values[key]; !ok || value == nil {
			diags.AddAttributeError(at.AtMapKey(key), "Missing required config key",
				fmt.Sprintf("The connector requires %q.", key))
		}
	}

	validateConditions(values, schema, at, diags)

	for _, key := range sortedKeys(values) {
		prop := schema.Properties[key]
		if prop == nil || values[key] == nil {
			continue
		}

		if prop.ReadOnly {
			diags.AddAttributeError(at.AtMapKey(key), "Read-only config key",
				fmt.Sprintf("Hevo sets %q itself; remove it from config.", key))

			continue
		}

		validateValue(key, values[key], prop, at.AtMapKey(key), diags)
	}
}

func validateValue(key string, value any, prop *Property, at path.Path, diags *diag.Diagnostics) {
	if !hasType(value, prop.Type) {
		diags.AddAttributeError(at, "Wrong config value type",
			fmt.Sprintf("Expected %s, got %v.", prop.Type, value))

		return
	}

	if len(prop.Enum) > 0 && !slices.ContainsFunc(prop.Enum, func(allowed any) bool { return ScalarsEqual(allowed, value) }) {
		allowed := make([]string, 0, len(prop.Enum))
		for _, e := range prop.Enum {
			allowed = append(allowed, fmt.Sprint(e))
		}

		diags.AddAttributeError(at, "Config value not allowed",
			fmt.Sprintf("Got %v; allowed values: %s.", value, strings.Join(allowed, ", ")))
	}

	if text, ok := value.(string); ok && prop.Pattern != "" {
		// A pattern Go cannot compile is left to the API.
		if re, err := regexp.Compile(prop.Pattern); err == nil && !re.MatchString(text) {
			diags.AddAttributeError(at, "Config value does not match the pattern",
				fmt.Sprintf("'%s' must match `%s`; got %q.", key, prop.Pattern, text))
		}
	}

	if nested, ok := value.(map[string]any); ok {
		validateObject(nested, prop, at, diags)
	}
}

// hasType reports whether value fits a JSON Schema type. A string is accepted for a number or a
// boolean, as Hevo accepts both forms.
func hasType(value any, jsonType string) bool {
	switch jsonType {
	case "string":
		_, ok := value.(string)

		return ok
	case "integer":
		switch v := value.(type) {
		case int64:
			return true
		case string:
			_, ok := parseInt(v)

			return ok
		}

		return false
	case "number":
		switch value.(type) {
		case int64, float64, string:
			return true
		}

		return false
	case "boolean":
		switch v := value.(type) {
		case bool:
			return true
		case string:
			return v == "true" || v == "false"
		}

		return false
	case "object":
		_, ok := value.(map[string]any)

		return ok
	case "array":
		_, ok := value.([]any)

		return ok
	default:
		return true
	}
}

func parseInt(s string) (int64, bool) {
	var i int64

	_, err := fmt.Sscan(s, &i)

	return i, err == nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
