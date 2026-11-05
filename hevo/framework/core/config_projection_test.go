package core

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

func testSchema() *Property {
	return &Property{
		Type:     "object",
		Required: []string{"host"},
		// mode A, its default, requires secret.
		Conditions: []Condition{{When: map[string]any{"mode": "A"}, Required: []string{"secret"}}},
		Properties: map[string]*Property{
			"host":     {Type: "string"},
			"port":     {Type: "integer"},
			"password": {Type: "string", WriteOnly: true},
			"external": {Type: "string", ReadOnly: true},
			"mode":     {Type: "string", Enum: []any{"A", "B"}, Default: "A"},
			"db":       {Type: "string", Pattern: "^[A-Za-z][A-Za-z0-9_]*$"},
			"secret":   {Type: "string"},
			"stage":    {Type: "boolean", Default: false},
			"ssh": {Type: "object", Properties: map[string]*Property{
				"host": {Type: "string"},
				"key":  {Type: "string", WriteOnly: true},
			}},
		},
	}
}

func TestProjectConfig(t *testing.T) {
	t.Parallel()

	remote := map[string]any{
		"host":     "db.acme.com",
		"port":     float64(5432),
		"password": "******",
		"region":   "us-east-1",
		"ssh":      map[string]any{"host": "bastion.acme.com", "key": "******", "user": "hevo"},
	}

	tests := []struct {
		name     string
		mask     map[string]any
		expected map[string]any
	}{
		{
			name: "keeps only managed keys and local credentials",
			mask: map[string]any{
				"host":     "db.acme.com",
				"port":     "5432",
				"password": "s3cret",
				"ssh":      map[string]any{"host": "bastion.acme.com", "key": "pem"},
			},
			expected: map[string]any{
				"host":     "db.acme.com",
				"port":     "5432",
				"password": "s3cret",
				"ssh":      map[string]any{"host": "bastion.acme.com", "key": "pem"},
			},
		},
		{
			name:     "remote change is drift",
			mask:     map[string]any{"host": "old.acme.com"},
			expected: map[string]any{"host": "db.acme.com"},
		},
		{
			name:     "key gone remotely is null",
			mask:     map[string]any{"mode": "B"},
			expected: map[string]any{"mode": nil},
		},
		{
			name:     "key Hevo does not return keeps its default",
			mask:     map[string]any{"stage": false},
			expected: map[string]any{"stage": false},
		},
		{
			name:     "key Hevo does not return is null when not its default",
			mask:     map[string]any{"stage": true},
			expected: map[string]any{"stage": nil},
		},
		{
			name: "import keeps every key except credentials",
			mask: nil,
			expected: map[string]any{
				"host":   "db.acme.com",
				"port":   float64(5432),
				"region": "us-east-1",
				"ssh":    map[string]any{"host": "bastion.acme.com", "user": "hevo"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ProjectConfig(remote, tt.mask, testSchema()); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ProjectConfig() = %#v, want %#v", got, tt.expected)
			}
		})
	}
}

func TestProjectConfigWithoutSchemaRecognisesMaskedValues(t *testing.T) {
	t.Parallel()

	got := ProjectConfig(map[string]any{"password": "********"}, map[string]any{"password": "s3cret"}, nil)
	if got["password"] != "s3cret" {
		t.Errorf("password = %v, want the local value", got["password"])
	}
}

func TestWithoutCurrentValues(t *testing.T) {
	t.Parallel()

	remote := map[string]any{"host": "a", "port": float64(1), "password": "******"}
	changed := map[string]any{"host": "a", "port": "2", "password": "x", "stage": false, "mode": "B"}

	expected := map[string]any{"port": "2", "password": "x", "mode": "B"}
	if got := WithoutCurrentValues(changed, remote, testSchema()); !reflect.DeepEqual(got, expected) {
		t.Errorf("WithoutCurrentValues() = %v, want %v", got, expected)
	}
}

func TestChangedConfig(t *testing.T) {
	t.Parallel()

	state := map[string]any{"host": "a", "port": int64(1), "password": "x"}
	plan := map[string]any{"host": "b", "port": int64(1), "password": "x", "mode": "A"}

	expected := map[string]any{"host": "b", "mode": "A"}
	if got := ChangedConfig(plan, state); !reflect.DeepEqual(got, expected) {
		t.Errorf("ChangedConfig() = %v, want %v", got, expected)
	}
}

func TestDynamicRoundTrip(t *testing.T) {
	t.Parallel()

	in := map[string]any{"s": "x", "n": int64(3), "f": 1.5, "b": true, "o": map[string]any{"k": "v"}, "l": []any{"a", int64(1)}}

	dyn, diags := MapToDynamic(in)
	if diags.HasError() {
		t.Fatal(diags)
	}

	out, diags := DynamicToMap(dyn)
	if diags.HasError() {
		t.Fatal(diags)
	}

	if !reflect.DeepEqual(out, in) {
		t.Errorf("round trip = %#v, want %#v", out, in)
	}
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   map[string]any
		expected []string
	}{
		{name: "valid", config: map[string]any{"host": "h", "port": "5432", "mode": "A", "secret": "s", "ssh": map[string]any{"host": "b"}}},
		{name: "required by a condition", config: map[string]any{"host": "h", "mode": "A"}, expected: []string{"Missing required config key"}},
		{name: "required by a condition through its default", config: map[string]any{"host": "h"}, expected: []string{"Missing required config key"}},
		{name: "condition not met", config: map[string]any{"host": "h", "mode": "B"}},
		{name: "pattern", config: map[string]any{"host": "h", "mode": "B", "db": "1-not-valid"}, expected: []string{"Config value does not match the pattern"}},
		{name: "pattern matched", config: map[string]any{"host": "h", "mode": "B", "db": "HEVO_DB"}},
		{name: "unknown key", config: map[string]any{"host": "h", "mode": "B", "hots": "x"}, expected: []string{"Unknown config key"}},
		{name: "missing required", config: map[string]any{"port": int64(1), "mode": "B"}, expected: []string{"Missing required config key"}},
		{name: "wrong type", config: map[string]any{"host": "h", "mode": "B", "port": "five"}, expected: []string{"Wrong config value type"}},
		{name: "not allowed", config: map[string]any{"host": "h", "mode": "C"}, expected: []string{"Config value not allowed"}},
		{name: "read-only", config: map[string]any{"host": "h", "mode": "B", "external": "x"}, expected: []string{"Read-only config key"}},
		{name: "nested unknown", config: map[string]any{"host": "h", "mode": "B", "ssh": map[string]any{"hst": "b"}}, expected: []string{"Unknown config key"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var diags diag.Diagnostics

			ValidateConfig(tt.config, testSchema(), path.Root("config"), &diags)

			var got []string
			for _, d := range diags.Errors() {
				got = append(got, d.Summary())
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("errors = %v, want %v", got, tt.expected)
			}
		})
	}
}
