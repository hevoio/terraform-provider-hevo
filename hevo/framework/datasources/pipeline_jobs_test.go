package datasources_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	sdkmock "github.com/hevoio/hevo-go/tests/mock"

	"github.com/hevoio/terraform-provider-hevo/hevo/tests/mock"
)

func jobJSON(id string) string {
	return fmt.Sprintf(`{"id": %q, "type": "INCREMENTAL", "status": "COMPLETED", "created_at": 1760000000000,
		"updated_at": 1760000060000, "events_ingested": 10, "events_loaded": 9, "events_failed": 1,
		"objects_success": 2, "duration": 60000, "latency": {"min": 100, "max": 900, "mean": 400}}`, id)
}

func TestPipelineJobsDatasourceMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	client.When(http.MethodGet, "/api/v2/pipelines/42/jobs").ThenCall(func(req *http.Request) (*http.Response, error) {
		q := req.URL.Query()
		if q.Get("cursor") == "" {
			if q.Get("limit") != "3" || strings.Join(q["type"], ",") != "INCREMENTAL" {
				t.Errorf("first page query = %v", q)
			}

			return sdkmock.NewResponse(req, http.StatusOK, `{"code": "Success", "has_more": true, "next_cursor": "p2",
				"data": [`+jobJSON("j-3")+`, `+jobJSON("j-2")+`]}`), nil
		}

		if q.Get("limit") != "1" {
			t.Errorf("second page limit = %q, want the 1 job still wanted", q.Get("limit"))
		}

		return sdkmock.NewResponse(req, http.StatusOK, `{"code": "Success", "has_more": true, "next_cursor": "p3",
			"data": [`+jobJSON("j-1")+`]}`), nil
	})
	client.When(http.MethodGet, "/api/v2/pipelines/42/jobs/j-3").ThenReply(http.StatusOK, `{"code": "Success", "data": `+jobJSON("j-3")+`}`)
	client.When(http.MethodGet, "/api/v2/pipelines/42/jobs/j-3/objects").ThenReply(http.StatusOK, `{"code": "Success", "has_more": false,
		"data": [{"id": "o-1", "object_name": "public.orders", "status": "COMPLETED", "events_loaded": 9, "offset": null}]}`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mock.ProviderConfig + `
data "hevo_pipeline_jobs" "recent" {
  pipeline_id = "42"
  types       = ["INCREMENTAL"]
  limit       = 3
}

data "hevo_pipeline_jobs" "one" {
  pipeline_id = "42"
  job_id      = "j-3"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.hevo_pipeline_jobs.recent", "jobs.#", "3"),
					resource.TestCheckResourceAttr("data.hevo_pipeline_jobs.recent", "jobs.0.id", "j-3"),
					resource.TestCheckResourceAttr("data.hevo_pipeline_jobs.recent", "jobs.2.id", "j-1"),
					resource.TestCheckResourceAttr("data.hevo_pipeline_jobs.recent", "jobs.0.events_loaded", "9"),
					resource.TestCheckResourceAttr("data.hevo_pipeline_jobs.recent", "jobs.0.latency_mean_ms", "400"),
					resource.TestCheckResourceAttr("data.hevo_pipeline_jobs.recent", "jobs.0.created_at", "2025-10-09T08:53:20Z"),
					resource.TestCheckResourceAttr("data.hevo_pipeline_jobs.recent", "objects.#", "0"),
					resource.TestCheckResourceAttr("data.hevo_pipeline_jobs.one", "jobs.#", "1"),
					resource.TestCheckResourceAttr("data.hevo_pipeline_jobs.one", "objects.0.object_name", "public.orders"),
					resource.TestCheckResourceAttr("data.hevo_pipeline_jobs.one", "objects.0.events_loaded", "9"),
					resource.TestCheckNoResourceAttr("data.hevo_pipeline_jobs.one", "objects.0.events_ingested"),
				),
			},
		},
	})
}
