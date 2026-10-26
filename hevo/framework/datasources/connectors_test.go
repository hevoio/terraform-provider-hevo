package datasources_test

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	sdkmock "github.com/hevoio/hevo-go/tests/mock"

	"github.com/hevoio/terraform-provider-hevo/hevo/tests/mock"
)

func TestConnectorsDatasourceFollowsPagesMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	client.When(http.MethodGet, "/api/v2/metadata/sources/connectors").ThenCall(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Query().Get("limit"); got != "500" {
			t.Errorf("limit = %q, want 500", got)
		}

		if req.URL.Query().Get("cursor") == "" {
			return sdkmock.NewResponse(req, http.StatusOK, `{"code": "Success", "has_more": true, "next_cursor": "p2", "data": [
				{"connector_id": "hubspot", "type": "HUBSPOT", "display_name": "HubSpot", "description": "CRM", "in_beta": false}]}`), nil
		}

		return sdkmock.NewResponse(req, http.StatusOK, `{"code": "Success", "has_more": false, "next_cursor": null, "data": [
			{"connector_id": "stripe", "type": "STRIPE", "display_name": "Stripe", "description": null, "in_beta": true}]}`), nil
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mock.ProviderConfig + `data "hevo_connectors" "sources" { kind = "source" }`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.hevo_connectors.sources", "connectors.#", "2"),
					resource.TestCheckResourceAttr("data.hevo_connectors.sources", "connectors.0.connector_id", "hubspot"),
					resource.TestCheckResourceAttr("data.hevo_connectors.sources", "connectors.0.description", "CRM"),
					resource.TestCheckResourceAttr("data.hevo_connectors.sources", "connectors.1.type", "STRIPE"),
					resource.TestCheckResourceAttr("data.hevo_connectors.sources", "connectors.1.in_beta", "true"),
					resource.TestCheckNoResourceAttr("data.hevo_connectors.sources", "connectors.1.description"),
				),
			},
		},
	})
}

func TestConnectorsDatasourceFiltersMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	client.When(http.MethodGet, "/api/v2/metadata/sources/connectors").ThenCall(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Query().Get("type"); got != "HUBSPOT" {
			t.Errorf("type = %q, want HUBSPOT", got)
		}

		return sdkmock.NewResponse(req, http.StatusOK, `{"code": "Success", "has_more": false, "data": [
			{"connector_id": "hubspot", "type": "HUBSPOT", "display_name": "HubSpot"}]}`), nil
	})
	client.When(http.MethodGet, "/api/v2/metadata/destinations/connectors").ThenReply(http.StatusOK,
		`{"code": "Success", "has_more": false, "data": [{"connector_id": "snowflake", "type": "SNOWFLAKE", "display_name": "Snowflake"}]}`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mock.ProviderConfig + `
data "hevo_connectors" "bad" {
  kind = "destination"
  type = "SNOWFLAKE"
}
`,
				ExpectError: regexp.MustCompile(`type filters source connectors only`),
			},
			{
				Config:      mock.ProviderConfig + `data "hevo_connectors" "bad" { kind = "sources" }`,
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
			{
				Config: mock.ProviderConfig + `
data "hevo_connectors" "hubspot" {
  kind = "source"
  type = "HUBSPOT"
}
data "hevo_connectors" "destinations" { kind = "destination" }
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.hevo_connectors.hubspot", "connectors.#", "1"),
					resource.TestCheckResourceAttr("data.hevo_connectors.destinations", "connectors.0.connector_id", "snowflake"),
				),
			},
		},
	})
}
