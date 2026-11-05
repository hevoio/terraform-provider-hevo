package resources

import (
	"context"
	"maps"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// pipelineValue builds a hevo_pipeline value with the given string attributes; the rest are null.
func pipelineValue(t *testing.T, r resource.Resource, values map[string]string) (tftypes.Value, resource.SchemaResponse) {
	t.Helper()

	var schemaResp resource.SchemaResponse

	r.Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)

	objectType, ok := schemaResp.Schema.Type().TerraformType(context.Background()).(tftypes.Object)
	if !ok {
		t.Fatal("pipeline schema is not an object")
	}

	attrs := map[string]tftypes.Value{}
	for name, attrType := range objectType.AttributeTypes {
		attrs[name] = tftypes.NewValue(attrType, nil)
	}

	for name, value := range values {
		attrs[name] = tftypes.NewValue(tftypes.String, value)
	}

	return tftypes.NewValue(objectType, attrs), schemaResp
}

func TestPipelineModifyPlanWarnsOnReplacement(t *testing.T) {
	t.Parallel()

	base := map[string]string{
		"id": "42", "source_connector_id": "hubspot", "destination_id": "7",
		"destination_prefix": "hubspot_", "replication_type": "HISTORICAL_AND_INCREMENTAL", "name": "p",
	}

	tests := []struct {
		name     string
		change   map[string]string
		expected string
	}{
		{name: "prefix change warns", change: map[string]string{"destination_prefix": "crm_"}, expected: "destination_prefix"},
		{name: "rename does not warn", change: map[string]string{"name": "renamed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &pipeline{}
			planned := map[string]string{}

			maps.Copy(planned, base)

			maps.Copy(planned, tt.change)

			stateValue, schemaResp := pipelineValue(t, r, base)
			planValue, _ := pipelineValue(t, r, planned)

			req := resource.ModifyPlanRequest{
				State: tfsdk.State{Schema: schemaResp.Schema, Raw: stateValue},
				Plan:  tfsdk.Plan{Schema: schemaResp.Schema, Raw: planValue},
			}
			resp := resource.ModifyPlanResponse{Plan: req.Plan}

			r.ModifyPlan(context.Background(), req, &resp)

			warnings := resp.Diagnostics.Warnings()
			if tt.expected == "" {
				if len(warnings) != 0 {
					t.Fatalf("warnings = %v, want none", warnings)
				}

				return
			}

			if len(warnings) != 1 || !strings.Contains(warnings[0].Detail(), tt.expected) {
				t.Fatalf("warnings = %v, want one naming %s", warnings, tt.expected)
			}
		})
	}
}
