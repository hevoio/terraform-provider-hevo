package e2e_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestResourcePipelineE2E creates, edits, imports and destroys a Pipeline. Set
// HEVO_E2E_PIPELINE to {"source_connector_id": "...", "source_config": {...},
// "destination_id": "..."} with a Source Hevo can reach and an existing Destination.
func TestResourcePipelineE2E(t *testing.T) {
	t.Parallel()

	raw := os.Getenv("HEVO_E2E_PIPELINE")
	if raw == "" {
		t.Skip("set HEVO_E2E_PIPELINE to run the Pipeline acceptance test")
	}

	var spec struct {
		SourceConnectorID string         `json:"source_connector_id"`
		SourceConfig      map[string]any `json:"source_config"`
		DestinationID     string         `json:"destination_id"`
	}
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("HEVO_E2E_PIPELINE: %v", err)
	}

	config, err := json.Marshal(spec.SourceConfig)
	if err != nil {
		t.Fatal(err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	hcl := func(frequency int) string {
		return fmt.Sprintf(`
resource "hevo_pipeline" "test" {
  name                    = "tf-acc-%[1]s"
  source_name             = "tf-acc-%[1]s"
  source_connector_id     = %[2]q
  source_config           = jsondecode(%[3]q)
  destination_id          = %[4]q
  destination_prefix      = "tf_acc_%[1]s_"
  replication_type        = "INCREMENTAL_ONLY"
  load_mode               = "MERGE"
  schema_evolution        = "BLOCK_ALL"
  schedule                = { sync_type = "SCHEDULED", frequency_minutes = %[5]d }
  failure_handling_policy = { object_failure_level = "PARTIAL" }
}
`, suffix, spec.SourceConnectorID, string(config), spec.DestinationID, frequency)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: hcl(60),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("hevo_pipeline.test", "id"),
					resource.TestCheckResourceAttr("hevo_pipeline.test", "disabled", "true"),
				),
			},
			{
				Config: hcl(120),
				Check:  resource.TestCheckResourceAttr("hevo_pipeline.test", "schedule.frequency_minutes", "120"),
			},
			{
				ResourceName:            "hevo_pipeline.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"source_config", "source_auth", "timeouts"},
			},
		},
	})
}
