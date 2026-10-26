package datasources_test

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hevoio/terraform-provider-hevo/hevo/tests/mock"
)

const snowflakeMetadata = `{"code": "Success", "data": {"connector_id": "snowflake", "type": "SNOWFLAKE", "display_name": "Snowflake",
  "schema": {"type": "object", "properties": {"config": {"type": "object", "required": ["account_url", "username"], "properties": {
    "account_url": {"type": "string"},
    "username":    {"type": "string"},
    "password":    {"type": "string", "format": "password", "writeOnly": true},
    "private_key": {"type": "string", "format": "password", "writeOnly": true}
  }}}}}}`

func TestConnectorMetadataDatasourceMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	client.When(http.MethodGet, "/api/v2/metadata/destinations/connectors/snowflake").ThenReply(http.StatusOK, snowflakeMetadata)
	client.When(http.MethodGet, "/api/v2/metadata/destinations/connectors/nope").ThenReply(http.StatusNotFound,
		`{"code": "RESOURCE_NOT_FOUND", "message": "No connector nope."}`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mock.ProviderConfig + `
data "hevo_connector_metadata" "missing" {
  kind         = "destination"
  connector_id = "nope"
}
`,
				ExpectError: regexp.MustCompile(`Unknown connector`),
			},
			{
				Config: mock.ProviderConfig + `
data "hevo_connector_metadata" "sf" {
  kind         = "destination"
  connector_id = "snowflake"
}

output "config_keys" {
  value = join(",", sort(keys(jsondecode(data.hevo_connector_metadata.sf.schema_json).properties.config.properties)))
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.hevo_connector_metadata.sf", "type", "SNOWFLAKE"),
					resource.TestCheckResourceAttr("data.hevo_connector_metadata.sf", "display_name", "Snowflake"),
					resource.TestCheckResourceAttr("data.hevo_connector_metadata.sf", "required_config_keys.#", "2"),
					resource.TestCheckResourceAttr("data.hevo_connector_metadata.sf", "required_config_keys.0", "account_url"),
					resource.TestCheckResourceAttr("data.hevo_connector_metadata.sf", "write_only_config_keys.#", "2"),
					resource.TestCheckResourceAttr("data.hevo_connector_metadata.sf", "write_only_config_keys.0", "password"),
					resource.TestCheckResourceAttr("data.hevo_connector_metadata.sf", "write_only_config_keys.1", "private_key"),
					resource.TestCheckOutput("config_keys", "account_url,password,private_key,username"),
				),
			},
		},
	})
}
