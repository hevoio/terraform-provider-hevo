package e2e_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestDatasourceConnectorsE2E(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: `
data "hevo_connectors" "destinations" { kind = "destination" }

data "hevo_connector_metadata" "first" {
  kind         = "destination"
  connector_id = data.hevo_connectors.destinations.connectors[0].connector_id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.hevo_connectors.destinations", "connectors.0.connector_id"),
					resource.TestCheckResourceAttrSet("data.hevo_connector_metadata.first", "schema_json"),
				),
			},
		},
	})
}
