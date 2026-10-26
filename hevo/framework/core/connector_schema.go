package core

// Property is one field of a connector's config, read from the connector's JSON Schema.
type Property struct {
	// Type is the JSON Schema type: string, integer, number, boolean, object or array. Empty when
	// the schema does not say.
	Type string
	// WriteOnly marks a credential: accepted on write, masked on every read.
	WriteOnly bool
	// ReadOnly marks a value Hevo sets; it is never sent.
	ReadOnly bool
	// Enum lists the allowed values, from enum or a oneOf of const.
	Enum []any
	// Required lists the keys an object must always carry.
	Required []string
	// Properties are the fields of an object.
	Properties map[string]*Property
	// Items describes the elements of an array.
	Items *Property
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

	if required, ok := raw["required"].([]any); ok {
		for _, r := range required {
			if name, ok := r.(string); ok {
				p.Required = append(p.Required, name)
			}
		}
	}

	if properties, ok := raw["properties"].(map[string]any); ok {
		p.Properties = make(map[string]*Property, len(properties))
		for name, prop := range properties {
			if m, ok := prop.(map[string]any); ok {
				p.Properties[name] = parseProperty(m)
			}
		}
	}

	if items, ok := raw["items"].(map[string]any); ok {
		p.Items = parseProperty(items)
	}

	return p
}

// property returns the named field of an object property, or nil.
func (p *Property) property(name string) *Property {
	if p == nil {
		return nil
	}

	return p.Properties[name]
}
