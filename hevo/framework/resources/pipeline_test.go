package resources_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	sdkmock "github.com/hevoio/hevo-go/tests/mock"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
	"github.com/hevoio/terraform-provider-hevo/hevo/tests/mock"
)

func TestMain(m *testing.M) {
	core.PollInterval = time.Millisecond

	os.Exit(m.Run())
}

const hubspotSchema = `{"code": "Success", "data": {"connector_id": "hubspot", "type": "HUBSPOT", "display_name": "HubSpot",
  "schema": {"type": "object", "properties": {"config": {"type": "object", "required": ["account_id"], "properties": {
    "authentication_method":    {"type": "string", "oneOf": [{"const": "OAUTH"}, {"const": "PRIVATE_APP_TOKEN"}]},
    "account_id":               {"type": "integer"},
    "private_app_access_token": {"type": "string", "format": "password", "writeOnly": true}
  }}}}}}`

// fakePipelines is an in-memory Pipelines API: one Pipeline, id 42. Edits answer 409 once, then
// report Initializing for one read before settling.
type fakePipelines struct {
	mu            sync.Mutex
	exists        bool
	pipeline      map[string]any
	createBody    map[string]any
	settingsPatch map[string]any
	sourcePatch   map[string]any
	sourcePatches int
	conflicts     int
	busyReads     int
}

func (f *fakePipelines) set(key string, value any) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.pipeline[key] = value
}

func (f *fakePipelines) response(req *http.Request, status int, code string) *http.Response {
	p := maps.Clone(f.pipeline)
	if f.busyReads > 0 {
		f.busyReads--
		p["state"] = "Initializing"
	}

	data, _ := json.Marshal(p)

	return sdkmock.NewResponse(req, status, fmt.Sprintf(`{"code": %q, "data": %s}`, code, data))
}

func (f *fakePipelines) register(t *testing.T, client *sdkmock.HTTPClient) {
	t.Helper()

	client.When(http.MethodGet, "/api/v2/metadata/sources/connectors/hubspot").ThenReply(http.StatusOK, hubspotSchema)

	client.When(http.MethodPost, "/api/v2/pipelines").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		body := decode(t, req)
		source, _ := body["source"].(map[string]any)
		f.createBody, f.exists, f.busyReads = body, true, 1
		f.pipeline = map[string]any{
			"id": 42, "name": body["name"], "state": "Initializing", "disabled": true,
			"health":             map[string]any{"value": "HEALTHY"},
			"source":             map[string]any{"name": source["name"], "connector_id": source["connector_id"], "type": "HUBSPOT"},
			"destination_id":     body["destination_id"],
			"destination_prefix": body["destination_prefix"],
			"replication_type":   body["replication_type"],
			"load_mode":          body["load_mode"],
			"schema_evolution":   body["schema_evolution"],
			"schedule":           body["schedule"],
			// Hevo fills in a threshold the user did not set.
			"failure_handling_policy": map[string]any{"object_failure_level": "ALL_OR_NONE", "object_failure_threshold": 0},
			"latency_alert":           map[string]any{"enabled": false, "threshold_minutes": 60},
			"created_at":              1760000000000, "created_by": "ops@acme.com",
			"updated_at": 1760000000000, "updated_by": "ops@acme.com",
		}
		f.pipeline["state"] = "Disabled"

		return f.response(req, http.StatusOK, "Success"), nil
	})

	client.When(http.MethodGet, "/api/v2/pipelines/42").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		if !f.exists {
			return sdkmock.NewResponse(req, http.StatusNotFound, `{"code": "RESOURCE_NOT_FOUND", "message": "No Pipeline 42."}`), nil
		}

		return f.response(req, http.StatusOK, "Success"), nil
	})

	client.When(http.MethodPatch, "/api/v2/pipelines/42").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		if f.conflicts > 0 {
			f.conflicts--

			return sdkmock.NewResponse(req, http.StatusConflict, `{"code": "RESOURCE_CONFLICT", "message": "Edit in progress."}`), nil
		}

		f.settingsPatch = decode(t, req)
		maps.Copy(f.pipeline, f.settingsPatch)
		f.busyReads = 1

		return f.response(req, http.StatusAccepted, "Accepted"), nil
	})

	client.When(http.MethodPatch, "/api/v2/pipelines/42/sources").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		f.sourcePatch = decode(t, req)
		f.sourcePatches++

		if name, ok := f.sourcePatch["name"]; ok {
			source, _ := f.pipeline["source"].(map[string]any)
			source["name"] = name
		}

		f.busyReads = 1

		return f.response(req, http.StatusAccepted, "Accepted"), nil
	})

	client.When(http.MethodDelete, "/api/v2/pipelines/42").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		f.exists = false

		return f.response(req, http.StatusAccepted, "Accepted"), nil
	})
}

func pipelineConfig(name, frequency, token, loadMode string) string {
	return mock.ProviderConfig + fmt.Sprintf(`
resource "hevo_pipeline" "p" {
  name                = %q
  source_name         = "HubSpot"
  source_connector_id = "hubspot"
  source_config = {
    authentication_method    = "PRIVATE_APP_TOKEN"
    account_id               = 12345
    private_app_access_token = %q
  }
  destination_id     = "7"
  destination_prefix = "hubspot_"
  replication_type   = "HISTORICAL_AND_INCREMENTAL"
  load_mode          = %q
  schema_evolution   = "ALLOW_ALL"
  schedule = {
    sync_type         = "SCHEDULED"
    frequency_minutes = %s
  }
  failure_handling_policy = {
    object_failure_level = "ALL_OR_NONE"
  }
}
`, name, token, loadMode, frequency)
}

func TestPipelineResourceMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := &fakePipelines{}
	fake.register(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: pipelineConfig("HubSpot to Snowflake", "60", "pat-1", "MERGE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hevo_pipeline.p", "id", "42"),
					resource.TestCheckResourceAttr("hevo_pipeline.p", "state", "Disabled"),
					resource.TestCheckResourceAttr("hevo_pipeline.p", "disabled", "true"),
					resource.TestCheckResourceAttr("hevo_pipeline.p", "source_type", "HUBSPOT"),
					resource.TestCheckResourceAttr("hevo_pipeline.p", "source_config.private_app_access_token", "pat-1"),
					resource.TestCheckNoResourceAttr("hevo_pipeline.p", "failure_handling_policy.object_failure_threshold"),
					resource.TestCheckNoResourceAttr("hevo_pipeline.p", "latency_alert"),
					func(*terraformState) error {
						options, _ := fake.createBody["options"].(map[string]any)
						if options["create_disabled"] != true {
							return fmt.Errorf("create options = %v, want create_disabled", options)
						}

						return nil
					},
				),
			},
			{
				// Settings and one Source key change: the settings PATCH survives a 409, and each
				// PATCH carries only what changed.
				PreConfig: func() { fake.conflicts = 1 },
				Config:    pipelineConfig("HubSpot prod", "30", "pat-2", "MERGE"),
				Check: func(*terraformState) error {
					wantSettings := map[string]any{
						"name":     "HubSpot prod",
						"schedule": map[string]any{"sync_type": "SCHEDULED", "frequency_minutes": float64(30)},
					}
					if !reflect.DeepEqual(fake.settingsPatch, wantSettings) {
						return fmt.Errorf("settings PATCH = %v, want %v", fake.settingsPatch, wantSettings)
					}

					wantSource := map[string]any{"config": map[string]any{"private_app_access_token": "pat-2"}}
					if !reflect.DeepEqual(fake.sourcePatch, wantSource) {
						return fmt.Errorf("Source PATCH = %v, want %v", fake.sourcePatch, wantSource)
					}

					return nil
				},
			},
			{
				// A setting changed in the Hevo UI shows as drift.
				PreConfig:          func() { fake.set("load_mode", "APPEND") },
				Config:             pipelineConfig("HubSpot prod", "30", "pat-2", "MERGE"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: pipelineConfig("HubSpot prod", "30", "pat-2", "MERGE"),
				Check: func(*terraformState) error {
					if fake.sourcePatches != 1 {
						return fmt.Errorf("Source PATCH calls = %d, want 1: unchanged config must not be resent", fake.sourcePatches)
					}

					return nil
				},
			},
			{
				ResourceName:      "hevo_pipeline.p",
				ImportState:       true,
				ImportStateVerify: true,
				// The Source's config and auth are never returned.
				ImportStateVerifyIgnore: []string{"source_config", "source_auth", "timeouts"},
			},
		},
	})
}

func TestPipelineResourceValidatesSourceConfigMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	(&fakePipelines{}).register(t, client)

	bad := mock.ProviderConfig + `
resource "hevo_pipeline" "p" {
  name                = "p"
  source_name         = "HubSpot"
  source_connector_id = "hubspot"
  source_config = {
    authentication_method = "API_KEY"
    acount_id             = 1
  }
  destination_id          = "7"
  destination_prefix      = "hubspot_"
  replication_type        = "INCREMENTAL_ONLY"
  load_mode               = "MERGE"
  schema_evolution        = "ALLOW_ALL"
  schedule                = { sync_type = "ON_DEMAND" }
  failure_handling_policy = { object_failure_level = "PARTIAL" }
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: bad, ExpectError: regexp.MustCompile(`Unknown config key`)},
			{Config: bad, ExpectError: regexp.MustCompile(`Missing required config key`)},
			{Config: bad, ExpectError: regexp.MustCompile(`Config value not allowed`)},
		},
	})
}
