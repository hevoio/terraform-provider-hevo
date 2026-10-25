package e2e_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestFlowE2E runs the whole flow in one apply: a Destination, a Pipeline into it, its objects,
// enabling it, then reading its jobs. It needs HEVO_E2E_DESTINATION and HEVO_E2E_PIPELINE (whose
// destination_id is ignored here), and HEVO_E2E_OBJECTS: a JSON list of object names to select.
func TestFlowE2E(t *testing.T) {
	t.Parallel()

	destinationRaw, pipelineRaw, objectsRaw := os.Getenv("HEVO_E2E_DESTINATION"), os.Getenv("HEVO_E2E_PIPELINE"), os.Getenv("HEVO_E2E_OBJECTS")
	if destinationRaw == "" || pipelineRaw == "" || objectsRaw == "" {
		t.Skip("set HEVO_E2E_DESTINATION, HEVO_E2E_PIPELINE and HEVO_E2E_OBJECTS to run the flow acceptance test")
	}

	var destination struct {
		ConnectorID string         `json:"connector_id"`
		Config      map[string]any `json:"config"`
	}

	var pipeline struct {
		SourceConnectorID string         `json:"source_connector_id"`
		SourceConfig      map[string]any `json:"source_config"`
	}

	var objects []string

	for raw, target := range map[string]any{destinationRaw: &destination, pipelineRaw: &pipeline, objectsRaw: &objects} {
		if err := json.Unmarshal([]byte(raw), target); err != nil {
			t.Fatalf("decoding %s: %v", raw, err)
		}
	}

	destinationConfig, _ := json.Marshal(destination.Config)
	sourceConfig, _ := json.Marshal(pipeline.SourceConfig)

	entries := make([]string, 0, len(objects))
	for _, name := range objects {
		entries = append(entries, fmt.Sprintf("    %q = {}", name))
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	config := fmt.Sprintf(`
resource "hevo_destination" "d" {
  name         = "tf-acc-%[1]s"
  connector_id = %[2]q
  config       = jsondecode(%[3]q)
  force_delete = true
}

resource "hevo_pipeline" "p" {
  name                    = "tf-acc-%[1]s"
  source_name             = "tf-acc-%[1]s"
  source_connector_id     = %[4]q
  source_config           = jsondecode(%[5]q)
  destination_id          = hevo_destination.d.id
  destination_prefix      = "tf_acc_%[1]s_"
  replication_type        = "HISTORICAL_AND_INCREMENTAL"
  load_mode               = "MERGE"
  schema_evolution        = "BLOCK_ALL"
  schedule                = { sync_type = "SCHEDULED", frequency_minutes = 60 }
  failure_handling_policy = { object_failure_level = "PARTIAL" }
}

resource "hevo_pipeline_objects" "o" {
  pipeline_id = hevo_pipeline.p.id
  select      = "listed"
  objects = {
%[6]s
  }
}

resource "hevo_pipeline_state" "s" {
  pipeline_id = hevo_pipeline.p.id
  paused      = false
  depends_on  = [hevo_pipeline_objects.o]
}

data "hevo_pipeline_jobs" "j" {
  pipeline_id = hevo_pipeline.p.id
  depends_on  = [hevo_pipeline_state.s]
}
`, suffix, destination.ConnectorID, string(destinationConfig), pipeline.SourceConnectorID, string(sourceConfig), strings.Join(entries, "\n"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hevo_pipeline_state.s", "paused", "false"),
					resource.TestCheckResourceAttr("hevo_pipeline_objects.o", "objects.%", strconv.Itoa(len(objects))),
				),
			},
			{
				// A second plan of the same configuration is empty.
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}
