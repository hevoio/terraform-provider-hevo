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

// A source connector has no type of its own; its variants and default are in the schema.
const mysqlMetadata = `{"code": "Success", "data": {"connector_id": "SOURCE_CONNECTOR_MYSQL", "display_name": "MySQL",
  "schema": {"type": "object", "properties": {
    "type": {"type": "string", "enum": ["AURORA", "AWS_RDS_MYSQL", "MYSQL"], "default": "MYSQL"},
    "config": {"type": "object", "properties": {"database_host": {"type": "string"}}}}}}}`

func TestConnectorMetadataDatasourceSourceTypesMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	client.When(http.MethodGet, "/api/v2/metadata/sources/connectors/SOURCE_CONNECTOR_MYSQL").ThenReply(http.StatusOK, mysqlMetadata)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config: mock.ProviderConfig + `
data "hevo_connector_metadata" "mysql" {
  kind         = "source"
  connector_id = "SOURCE_CONNECTOR_MYSQL"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.hevo_connector_metadata.mysql", "type", "MYSQL"),
				resource.TestCheckResourceAttr("data.hevo_connector_metadata.mysql", "source_types.#", "3"),
				resource.TestCheckResourceAttr("data.hevo_connector_metadata.mysql", "source_types.1", "AWS_RDS_MYSQL"),
			),
		}},
	})
}

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
					resource.TestCheckResourceAttr("data.hevo_connector_metadata.sf", "source_types.#", "0"),
				),
			},
		},
	})
}
