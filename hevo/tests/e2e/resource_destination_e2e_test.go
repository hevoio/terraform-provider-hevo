package e2e_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestResourceDestinationE2E creates, renames, imports and destroys a Destination. Set
// HEVO_E2E_DESTINATION to {"connector_id": "...", "config": {...}} with working credentials:
// Hevo tests the connection on create.
func TestResourceDestinationE2E(t *testing.T) {
	t.Parallel()

	raw := os.Getenv("HEVO_E2E_DESTINATION")
	if raw == "" {
		t.Skip("set HEVO_E2E_DESTINATION to run the Destination acceptance test")
	}

	var spec struct {
		ConnectorID string         `json:"connector_id"`
		Config      map[string]any `json:"config"`
	}
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("HEVO_E2E_DESTINATION: %v", err)
	}

	config, err := json.Marshal(spec.Config)
	if err != nil {
		t.Fatal(err)
	}

	name := fmt.Sprintf("tf-acc-%d", time.Now().UnixNano())
	hcl := func(name string) string {
		return fmt.Sprintf(`
resource "hevo_destination" "test" {
  name         = %q
  connector_id = %q
  config       = jsondecode(%q)
}
`, name, spec.ConnectorID, string(config))
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: hcl(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("hevo_destination.test", "id"),
					resource.TestCheckResourceAttr("hevo_destination.test", "name", name),
				),
			},
			{
				Config: hcl(name + "-renamed"),
				Check:  resource.TestCheckResourceAttr("hevo_destination.test", "name", name+"-renamed"),
			},
			{
				ResourceName:            "hevo_destination.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config", "force_delete"},
			},
		},
	})
}
