package core

import (
	"regexp"
	"slices"
)

// maskedName and maskedKeys are the keys Hevo masks in every read by their name, whatever their
// type, as its connector templates do (EntityTemplateConfig.maskedTemplateConfig). The connector
// schema marks only password-typed fields writeOnly, so these are marked here too.
var (
	maskedName = regexp.MustCompile(`(?i)^.*(password|secret_key|private_key|passphrase)$`)
	maskedKeys = []string{"ssl_client_key", "ssl_ca_file", "ssl_client_cert_file"}
)

func isMaskedByName(name string) bool {
	return maskedName.MatchString(name) || slices.Contains(maskedKeys, name)
}

// Property is one field of a connector's config, read from the connector's JSON Schema.
type Property struct {
	// Type is the JSON Schema type: string, integer, number, boolean, object or array. Empty when
	// the schema does not say.
	Type string
	// WriteOnly marks a credential: accepted on write, masked on every read.
	WriteOnly bool
	// ReadOnly marks a value Hevo sets; it is never sent.
	ReadOnly bool
	// Default is the value Hevo uses when the key is not sent, or nil.
	Default any
	// Pattern is the regular expression a string value must match, or empty.
	Pattern string
	// Enum lists the allowed values, from enum or a oneOf of const.
	Enum []any
	// Required lists the keys an object must always carry.
	Required []string
	// Properties are the fields of an object.
	Properties map[string]*Property
	// Items describes the elements of an array.
	Items *Property
	// Conditions are the object's if/then rules: keys required once other keys hold given values.
	Conditions []Condition
}

// Condition is an if/then rule of an object: when every key in When holds its value, the keys in
// Required must be set.
type Condition struct {
	When     map[string]any
	Required []string
}

// ParseConnectorConfigSchema returns the config property of a connector's request JSON Schema,
// as GET /api/v2/metadata/{sources,destinations}/connectors/{id} returns it. It returns nil when
// the schema has no config property.
func ParseConnectorConfigSchema(schema map[string]any) *Property {
	properties, _ := schema["properties"].(map[string]any)
	config, _ := properties["config"].(map[string]any)

	if config == nil {
		return nil
	}

	return parseProperty(config)
}

func parseProperty(raw map[string]any) *Property {
	p := &Property{}
	p.Type, _ = raw["type"].(string)
	p.WriteOnly, _ = raw["writeOnly"].(bool)
	p.ReadOnly, _ = raw["readOnly"].(bool)
	p.Default = raw["default"]
	p.Pattern, _ = raw["pattern"].(string)

	if enum, ok := raw["enum"].([]any); ok {
		p.Enum = enum
	}

	if oneOf, ok := raw["oneOf"].([]any); ok {
		for _, option := range oneOf {
			if o, ok := option.(map[string]any); ok {
				if c, ok := o["const"]; ok {
					p.Enum = append(p.Enum, c)
				}
			}
		}
	}

	if required := stringList(raw["required"]); len(required) > 0 {
		p.Required = required
	}

	if properties, ok := raw["properties"].(map[string]any); ok {
		p.Properties = make(map[string]*Property, len(properties))
		for name, prop := range properties {
			if m, ok := prop.(map[string]any); ok {
				p.Properties[name] = parseProperty(m)
				if isMaskedByName(name) {
					p.Properties[name].WriteOnly = true
				}
			}
		}
	}

	if items, ok := raw["items"].(map[string]any); ok {
		p.Items = parseProperty(items)
	}

	p.Conditions = parseConditions(raw)

	return p
}

// parseConditions returns the if/then rules of an object schema, its own and those under allOf.
// Only rules whose if compares keys to constants are kept; others are left to the API.
func parseConditions(raw map[string]any) []Condition {
	rules := []map[string]any{raw}

	if allOf, ok := raw["allOf"].([]any); ok {
		for _, r := range allOf {
			if m, ok := r.(map[string]any); ok {
				rules = append(rules, m)
			}
		}
	}

	var conditions []Condition

	for _, rule := range rules {
		ifPart, _ := rule["if"].(map[string]any)
		thenPart, _ := rule["then"].(map[string]any)
		properties, _ := ifPart["properties"].(map[string]any)
		required := stringList(thenPart["required"])

		if len(properties) == 0 || len(required) == 0 {
			continue
		}

		when := make(map[string]any, len(properties))

		for key, value := range properties {
			prop, _ := value.(map[string]any)

			constant, ok := prop["const"]
			if !ok {
				when = nil

				break
			}

			when[key] = constant
		}

		if when != nil {
			conditions = append(conditions, Condition{When: when, Required: required})
		}
	}

	return conditions
}

func stringList(raw any) []string {
	list, _ := raw.([]any)
	values := make([]string, 0, len(list))

	for _, v := range list {
		if s, ok := v.(string); ok {
			values = append(values, s)
		}
	}

	return values
}

// property returns the named field of an object property, or nil.
func (p *Property) property(name string) *Property {
	if p == nil {
		return nil
	}

	return p.Properties[name]
}
