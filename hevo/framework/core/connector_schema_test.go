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
