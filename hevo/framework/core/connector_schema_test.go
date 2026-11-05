package core

import (
	"encoding/json"
	"testing"
)

func TestParseConnectorConfigSchema(t *testing.T) {
	t.Parallel()

	var raw map[string]any
	if err := json.Unmarshal([]byte(`{"properties": {"config": {"type": "object", "required": ["account_id"], "properties": {
		"authentication_method": {"type": "string", "oneOf": [{"const": "OAUTH"}, {"const": "PRIVATE_APP_TOKEN"}]},
		"account_id": {"type": "integer"},
		"token": {"type": "string", "format": "password", "writeOnly": true},
		"tags": {"type": "array", "items": {"type": "string"}}
	}}}}`), &raw); err != nil {
		t.Fatal(err)
	}

	p := ParseConnectorConfigSchema(raw)

	if p == nil || len(p.Properties) != 4 || p.Required[0] != "account_id" {
		t.Fatalf("parsed = %+v", p)
	}

	if !p.Properties["token"].WriteOnly || len(p.Properties["authentication_method"].Enum) != 2 {
		t.Errorf("token = %+v, authentication_method = %+v", p.Properties["token"], p.Properties["authentication_method"])
	}

	if p.Properties["tags"].Items.Type != "string" {
		t.Errorf("tags items = %+v", p.Properties["tags"].Items)
	}

	if ParseConnectorConfigSchema(map[string]any{}) != nil {
		t.Error("want nil without a config property")
	}
}

func TestParseConnectorConfigSchemaConditions(t *testing.T) {
	t.Parallel()

	var raw map[string]any
	if err := json.Unmarshal([]byte(`{"properties": {"config": {"type": "object",
		"properties": {
			"auth": {"type": "string", "default": "BASIC"},
			"password": {"type": "string"},
			"warehouse": {"type": "string", "pattern": "^[A-Za-z]+$"}
		},
		"allOf": [
			{"if": {"required": ["auth"], "properties": {"auth": {"const": "BASIC"}}}, "then": {"required": ["password"]}},
			{"if": {"properties": {"auth": {"enum": ["A", "B"]}}}, "then": {"required": ["password"]}}
		]}}}`), &raw); err != nil {
		t.Fatal(err)
	}

	p := ParseConnectorConfigSchema(raw)

	// The rule whose if is not a const comparison is left to the API.
	if len(p.Conditions) != 1 || p.Conditions[0].When["auth"] != "BASIC" || p.Conditions[0].Required[0] != "password" {
		t.Errorf("conditions = %+v", p.Conditions)
	}

	if p.Properties["warehouse"].Pattern != "^[A-Za-z]+$" || p.Properties["auth"].Default != "BASIC" {
		t.Errorf("warehouse = %+v, auth = %+v", p.Properties["warehouse"], p.Properties["auth"])
	}
}

func TestParseConnectorConfigSchemaMasksByName(t *testing.T) {
	t.Parallel()

	var raw map[string]any
	if err := json.Unmarshal([]byte(`{"properties": {"config": {"type": "object", "properties": {
		"private_key":            {"type": "string"},
		"Database_Password":      {"type": "string"},
		"ssl_client_key":         {"type": "string"},
		"private_key_passphrase": {"type": "string", "format": "password", "writeOnly": true},
		"private_key_path":       {"type": "string"},
		"warehouse":              {"type": "string"}
	}}}}`), &raw); err != nil {
		t.Fatal(err)
	}

	p := ParseConnectorConfigSchema(raw)

	for name, want := range map[string]bool{
		"private_key": true, "Database_Password": true, "ssl_client_key": true, "private_key_passphrase": true,
		"private_key_path": false, "warehouse": false,
	} {
		if got := p.Properties[name].WriteOnly; got != want {
			t.Errorf("%s WriteOnly = %v, want %v", name, got, want)
		}
	}
}
