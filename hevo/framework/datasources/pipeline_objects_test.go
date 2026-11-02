package datasources_test

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	sdkmock "github.com/hevoio/hevo-go/tests/mock"

	"github.com/hevoio/terraform-provider-hevo/hevo/tests/mock"
)

func TestPipelineObjectsDatasourceMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	client.When(http.MethodGet, "/api/v2/pipelines/42/objects").ThenCall(func(req *http.Request) (*http.Response, error) {
		got := req.URL.Query()["status"]
		if len(got) == 1 && got[0] == "INACCESSIBLE" {
			return sdkmock.NewResponse(req, http.StatusOK, `{"code": "Success", "has_more": false, "data": [
				{"fully_qualified_name": "shop.bulk.t1", "name": "t1", "status": "INACCESSIBLE", "selected": false, "load_mode": "MERGE"}]}`), nil
		}

		want := []string{"ACTIVE", "INACTIVE", "RESETTING", "BLOCKED", "INCONSISTENT"}
		if len(got) != len(want) {
			t.Errorf("default statuses = %v, want %v", got, want)
		}

		return sdkmock.NewResponse(req, http.StatusOK, `{"code": "Success", "has_more": false, "data": [
			{"fully_qualified_name": "shop.public.orders", "name": "orders", "status": "ACTIVE", "selected": true, "load_mode": "MERGE"},
			{"fully_qualified_name": "shop.public.users", "name": "users", "status": "INACTIVE", "selected": false, "load_mode": "APPEND"}]}`), nil
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: mock.ProviderConfig + `
data "hevo_pipeline_objects" "selectable" { pipeline_id = "42" }

data "hevo_pipeline_objects" "inaccessible" {
  pipeline_id = "42"
  statuses    = ["INACCESSIBLE"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.hevo_pipeline_objects.selectable", "objects.#", "2"),
					resource.TestCheckResourceAttr("data.hevo_pipeline_objects.selectable", "objects.0.fully_qualified_name", "shop.public.orders"),
					resource.TestCheckResourceAttr("data.hevo_pipeline_objects.selectable", "objects.0.selected", "true"),
					resource.TestCheckResourceAttr("data.hevo_pipeline_objects.selectable", "objects.1.load_mode", "APPEND"),
					resource.TestCheckResourceAttr("data.hevo_pipeline_objects.inaccessible", "objects.#", "1"),
				),
			},
		},
	})
}
