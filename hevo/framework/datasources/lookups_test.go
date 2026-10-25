package datasources_test

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	sdkmock "github.com/hevoio/hevo-go/tests/mock"

	"github.com/hevoio/terraform-provider-hevo/hevo/tests/mock"
)

const destinationJSON = `{"id": 7, "name": "Snowflake", "connector_id": "snowflake", "type": "SNOWFLAKE", "status": "CREATED",
  "config": {"account_url": "acme.snowflakecomputing.com", "password": "******", "port": 443},
  "created_at": 1760000000000, "created_by": "ops@acme.com", "updated_at": 1760000000000, "updated_by": "ops@acme.com"}`

const pipelineJSON = `{"id": 42, "name": "HubSpot to Snowflake", "state": "Initialized", "disabled": false,
  "health": {"value": "HEALTHY"}, "source": {"name": "HubSpot", "connector_id": "hubspot", "type": "HUBSPOT"},
  "destination_id": 7, "destination_prefix": "hubspot_", "replication_type": "HISTORICAL_AND_INCREMENTAL",
  "load_mode": "MERGE", "schema_evolution": "ALLOW_ALL", "schedule": {"sync_type": "SCHEDULED", "frequency_minutes": 60},
  "created_at": 1760000000000, "created_by": "ops@acme.com", "updated_at": 1760000000000, "updated_by": "ops@acme.com"}`

func TestDestinationDatasourcesMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	client.When(http.MethodGet, "/api/v2/destinations/7").ThenReply(http.StatusOK, `{"code": "Success", "data": `+destinationJSON+`}`)
	client.When(http.MethodGet, "/api/v2/destinations").ThenCall(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("cursor") == "" {
			return sdkmock.NewResponse(req, http.StatusOK, `{"code": "Success", "has_more": true, "next_cursor": "p2", "data": [`+destinationJSON+`]}`), nil
		}

		return sdkmock.NewResponse(req, http.StatusOK, `{"code": "Success", "has_more": false, "data": [
			{"id": 8, "name": "BigQuery", "connector_id": "bigquery", "type": "BIGQUERY", "config": {}}]}`), nil
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mock.ProviderConfig + `
data "hevo_destination" "sf" { id = "7" }
data "hevo_destinations" "all" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.hevo_destination.sf", "name", "Snowflake"),
					resource.TestCheckResourceAttr("data.hevo_destination.sf", "config.account_url", "acme.snowflakecomputing.com"),
					resource.TestCheckResourceAttr("data.hevo_destination.sf", "config.port", "443"),
					resource.TestCheckResourceAttr("data.hevo_destination.sf", "created_at", "2025-10-09T08:53:20Z"),
					resource.TestCheckResourceAttr("data.hevo_destinations.all", "destinations.#", "2"),
					resource.TestCheckResourceAttr("data.hevo_destinations.all", "destinations.1.id", "8"),
					resource.TestCheckNoResourceAttr("data.hevo_destinations.all", "destinations.1.created_at"),
				),
			},
		},
	})
}

func TestPipelineDatasourcesMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	client.When(http.MethodGet, "/api/v2/pipelines/42").ThenReply(http.StatusOK, `{"code": "Success", "data": `+pipelineJSON+`}`)
	client.When(http.MethodGet, "/api/v2/pipelines").ThenCall(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		if q.Get("destination_id") != "7" || q.Get("connector_id") != "hubspot" || len(q["state"]) != 2 {
			t.Errorf("filters = %v", q)
		}

		return sdkmock.NewResponse(req, http.StatusOK, `{"code": "Success", "has_more": false, "data": [`+pipelineJSON+`]}`), nil
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mock.ProviderConfig + `
data "hevo_pipeline" "p" { id = "42" }
data "hevo_pipelines" "filtered" {
  states              = ["Initialized", "Disabled"]
  source_connector_id = "hubspot"
  destination_id      = "7"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.hevo_pipeline.p", "state", "Initialized"),
					resource.TestCheckResourceAttr("data.hevo_pipeline.p", "source_type", "HUBSPOT"),
					resource.TestCheckResourceAttr("data.hevo_pipeline.p", "destination_id", "7"),
					resource.TestCheckResourceAttr("data.hevo_pipeline.p", "frequency_minutes", "60"),
					resource.TestCheckResourceAttr("data.hevo_pipelines.filtered", "pipelines.#", "1"),
					resource.TestCheckResourceAttr("data.hevo_pipelines.filtered", "pipelines.0.name", "HubSpot to Snowflake"),
				),
			},
		},
	})
}
