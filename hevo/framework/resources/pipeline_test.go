package resources_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	sdkmock "github.com/hevoio/hevo-go/tests/mock"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
	"github.com/hevoio/terraform-provider-hevo/hevo/tests/mock"
)

func TestMain(m *testing.M) {
	core.PollInterval = time.Millisecond
	core.CreateRecoveryWait = 200 * time.Millisecond

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
	failCreate    int  // the status the next create answers with instead of 200
	createdAnyway bool // whether that failed create still made the Pipeline
	hiddenLists   int  // lists that miss the Pipeline, as while a create is still finishing
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

// storedSchedule is the schedule Hevo stores: a frequency sent with ON_DEMAND is dropped.
func storedSchedule(sent any) any {
	schedule, ok := sent.(map[string]any)
	if !ok || schedule["sync_type"] != "ON_DEMAND" {
		return sent
	}

	return map[string]any{"sync_type": "ON_DEMAND"}
}

// failureHandlingPolicy is the policy Hevo stores for a create: the one sent, with a threshold of
// 0 when the user did not set one.
func failureHandlingPolicy(body map[string]any) map[string]any {
	policy := map[string]any{"object_failure_threshold": 0}
	if sent, ok := body["failure_handling_policy"].(map[string]any); ok {
		maps.Copy(policy, sent)
	}

	return policy
}

func (f *fakePipelines) register(t *testing.T, client *sdkmock.HTTPClient) {
	t.Helper()

	client.When(http.MethodGet, "/api/v2/metadata/sources/connectors/hubspot").ThenReply(http.StatusOK, hubspotSchema)

	client.When(http.MethodPost, "/api/v2/pipelines").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		body := decode(t, req)
		source, _ := body["source"].(map[string]any)

		sourceType, ok := source["type"].(string)
		if !ok {
			// Without a type, Hevo uses the connector's default.
			sourceType = "HUBSPOT"
		}

		f.createBody, f.exists, f.busyReads = body, true, 1
		f.pipeline = map[string]any{
			"id": 42, "name": body["name"], "state": "Initializing", "disabled": true,
			"health":                  map[string]any{"value": "HEALTHY"},
			"source":                  map[string]any{"name": source["name"], "connector_id": source["connector_id"], "type": sourceType},
			"destination_id":          body["destination_id"],
			"destination_prefix":      body["destination_prefix"],
			"replication_type":        body["replication_type"],
			"load_mode":               body["load_mode"],
			"schema_evolution":        body["schema_evolution"],
			"schedule":                storedSchedule(body["schedule"]),
			"failure_handling_policy": failureHandlingPolicy(body),
			"latency_alert":           map[string]any{"enabled": false, "threshold_minutes": 60},
			"created_at":              1760000000000, "created_by": "ops@acme.com",
			"updated_at": 1760000000000, "updated_by": "ops@acme.com",
		}
		f.pipeline["state"] = "Disabled"

		if f.failCreate != 0 {
			status := f.failCreate
			f.failCreate, f.exists = 0, f.createdAnyway

			return sdkmock.NewResponse(req, status, `{"code": "GATEWAY_TIMEOUT", "message": "Gateway timeout."}`), nil
		}

		return f.response(req, http.StatusOK, "Success"), nil
	})

	client.When(http.MethodGet, "/api/v2/pipelines").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		data := "[]"

		if f.hiddenLists > 0 {
			f.hiddenLists--
		} else if f.exists {
			p, _ := json.Marshal(f.pipeline)
			data = "[" + string(p) + "]"
		}

		return sdkmock.NewResponse(req, http.StatusOK, `{"code": "Success", "has_more": false, "data": `+data+`}`), nil
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

		if schedule, ok := f.settingsPatch["schedule"]; ok {
			f.pipeline["schedule"] = storedSchedule(schedule)
		}

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
					resource.TestCheckResourceAttr("hevo_pipeline.p", "source_type", "HUBSPOT"),
					resource.TestCheckResourceAttr("hevo_pipeline.p", "source_config.private_app_access_token", "pat-1"),
					resource.TestCheckNoResourceAttr("hevo_pipeline.p", "failure_handling_policy.object_failure_threshold"),
					resource.TestCheckNoResourceAttr("hevo_pipeline.p", "latency_alert"),
					func(*terraformState) error {
						options, _ := fake.createBody["options"].(map[string]any)
						if options["create_disabled"] != true {
							return fmt.Errorf("create options = %v, want create_disabled", options)
						}

						if source, _ := fake.createBody["source"].(map[string]any); source["type"] != nil {
							return fmt.Errorf("create source = %v, want no type when source_type is not set", source)
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

func TestPipelineResourceSourceTypeMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := &fakePipelines{}
	fake.register(t, client)

	withType := func(sourceType string) string {
		return strings.Replace(pipelineConfig("HubSpot", "60", "pat-1", "MERGE"),
			`source_connector_id = "hubspot"`, `source_connector_id = "hubspot"
  source_type         = "`+sourceType+`"`, 1)
	}
	sentType := func(want string) func(*terraformState) error {
		return func(*terraformState) error {
			if source, _ := fake.createBody["source"].(map[string]any); source["type"] != want {
				return fmt.Errorf("create source = %v, want type %q", source, want)
			}

			return nil
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: withType("HUBSPOT_EU"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hevo_pipeline.p", "source_type", "HUBSPOT_EU"),
					sentType("HUBSPOT_EU"),
				),
			},
			{
				// Hevo cannot change a Source's type, so a new one replaces the Pipeline.
				Config: withType("HUBSPOT"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("hevo_pipeline.p", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: sentType("HUBSPOT"),
			},
			{
				// Removing the attribute keeps the Pipeline.
				Config: pipelineConfig("HubSpot", "60", "pat-1", "MERGE"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr("hevo_pipeline.p", "source_type", "HUBSPOT"),
			},
		},
	})
}

func TestPipelineResourceFailureThresholdMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := &fakePipelines{}
	fake.register(t, client)

	withThreshold := func(threshold string) string {
		return strings.Replace(pipelineConfig("HubSpot", "60", "pat-1", "MERGE"),
			`object_failure_level = "ALL_OR_NONE"`, `object_failure_level = "PARTIAL"
    object_failure_threshold = `+threshold, 1)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: withThreshold("5"),
				Check:  resource.TestCheckResourceAttr("hevo_pipeline.p", "failure_handling_policy.object_failure_threshold", "5"),
			},
			{
				// Hevo cannot change the threshold after create, so a new one replaces the Pipeline.
				Config: withThreshold("6"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("hevo_pipeline.p", plancheck.ResourceActionDestroyBeforeCreate),
					},
				},
				Check: resource.TestCheckResourceAttr("hevo_pipeline.p", "failure_handling_policy.object_failure_threshold", "6"),
			},
			{
				// PARTIAL needs the threshold in the configuration.
				Config:      strings.Replace(withThreshold("6"), "    object_failure_threshold = 6\n", "", 1),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`should\s+be\s+greater\s+than\s+0`),
			},
			{
				Config: strings.Replace(withThreshold("6"), `"PARTIAL"`, `"ALL_OR_NONE"`, 1),
			},
			{
				// Removing it keeps the Pipeline, and Hevo keeps the threshold, so nothing is sent.
				PreConfig: func() { fake.settingsPatch = nil },
				Config:    pipelineConfig("HubSpot", "60", "pat-1", "MERGE"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("hevo_pipeline.p", plancheck.ResourceActionUpdate),
					},
				},
				Check: func(*terraformState) error {
					if fake.settingsPatch != nil {
						return fmt.Errorf("settings PATCH = %v, want none", fake.settingsPatch)
					}

					return nil
				},
			},
		},
	})
}

func TestPipelineResourceOnDemandKeepsFrequencyMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := &fakePipelines{}
	fake.register(t, client)

	scheduled := pipelineConfig("HubSpot", "60", "pat-1", "MERGE")
	onDemand := strings.Replace(scheduled, `sync_type         = "SCHEDULED"`, `sync_type         = "ON_DEMAND"`, 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: scheduled},
			{
				// Hevo drops the frequency; state keeps the configured one, so the plan settles.
				Config: onDemand,
				Check:  resource.TestCheckResourceAttr("hevo_pipeline.p", "schedule.frequency_minutes", "60"),
			},
			{
				Config: scheduled,
				Check: func(*terraformState) error {
					expected := map[string]any{"sync_type": "SCHEDULED", "frequency_minutes": float64(60)}
					if !reflect.DeepEqual(fake.settingsPatch["schedule"], expected) {
						return fmt.Errorf("schedule PATCH = %v, want %v", fake.settingsPatch["schedule"], expected)
					}

					return nil
				},
			},
		},
	})
}

func TestPipelineResourceValidatesFrequencyMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	(&fakePipelines{}).register(t, client)

	withoutFrequency := strings.Replace(pipelineConfig("HubSpot", "60", "pat-1", "MERGE"), "    frequency_minutes = 60\n", "", 1)
	required := regexp.MustCompile(`frequency_minutes\s+field\s+is\s+required`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: withoutFrequency, PlanOnly: true, ExpectError: required},
			{Config: pipelineConfig("HubSpot", "0", "pat-1", "MERGE"), PlanOnly: true, ExpectError: required},
			{
				Config:             strings.Replace(withoutFrequency, `"SCHEDULED"`, `"ON_DEMAND"`, 1),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestPipelineResourceValidatesLatencyAlertMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	(&fakePipelines{}).register(t, client)

	// Frequency 60.
	withAlert := func(alert string) string {
		return strings.Replace(pipelineConfig("HubSpot", "60", "pat-1", "MERGE"),
			"  failure_handling_policy = {", "  latency_alert = "+alert+"\n  failure_handling_policy = {", 1)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      withAlert(`{ enabled = true, threshold_minutes = 90 }`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`can\s+not\s+be\s+less\s+than\s+twice\s+the\s+sync\s+frequency`),
			},
			{
				Config:      withAlert(`{ enabled = true, threshold_minutes = 30000 }`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`greater\s+than\s+5\s+minutes\s+and\s+less\s+than\s+15\s+days`),
			},
			// Twice the frequency, a disabled alert and no alert all pass.
			{Config: withAlert(`{ enabled = true, threshold_minutes = 120 }`), PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: withAlert(`{ enabled = false, threshold_minutes = 90 }`), PlanOnly: true, ExpectNonEmptyPlan: true},
			{Config: pipelineConfig("HubSpot", "60", "pat-1", "MERGE"), PlanOnly: true, ExpectNonEmptyPlan: true},
		},
	})
}

func TestPipelineResourceValidatesFailureThresholdMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	(&fakePipelines{}).register(t, client)

	withPolicy := func(policy string) string {
		return strings.Replace(pipelineConfig("HubSpot", "60", "pat-1", "MERGE"),
			`object_failure_level = "ALL_OR_NONE"`, policy, 1)
	}
	invalid := regexp.MustCompile(`should\s+be\s+greater\s+than\s+0`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: withPolicy(`object_failure_level = "PARTIAL"`), PlanOnly: true, ExpectError: invalid},
			{
				Config:      withPolicy("object_failure_level = \"PARTIAL\"\n    object_failure_threshold = 0"),
				PlanOnly:    true,
				ExpectError: invalid,
			},
			{
				Config:             withPolicy("object_failure_level = \"PARTIAL\"\n    object_failure_threshold = 5"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{Config: withPolicy(`object_failure_level = "ALL_OR_NONE"`), PlanOnly: true, ExpectNonEmptyPlan: true},
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
  failure_handling_policy = { object_failure_level = "ALL_OR_NONE" }
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

// withoutSourceConfig drops the source_config block from a pipelineConfig.
func withoutSourceConfig(config string) string {
	start := strings.Index(config, "  source_config = {")
	end := start + strings.Index(config[start:], "\n  }\n") + len("\n  }\n")

	return config[:start] + config[end:]
}

func TestPipelineResourceRequiresSourceConfigToCreateMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	(&fakePipelines{}).register(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{{
			Config:      withoutSourceConfig(pipelineConfig("HubSpot", "60", "pat-1", "MERGE")),
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`Source config\(\x60\$\.source\.config\x60\) is required`),
		}},
	})
}

func TestPipelineResourceImportWithoutSourceConfigMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := &fakePipelines{}
	fake.register(t, client)

	// A Pipeline built in the Hevo UI, with a latency alert on.
	fake.exists = true
	fake.pipeline = map[string]any{
		"id": 42, "name": "HubSpot prod", "state": "Disabled", "disabled": true,
		"health":             map[string]any{"value": "HEALTHY"},
		"source":             map[string]any{"name": "HubSpot", "connector_id": "hubspot", "type": "HUBSPOT"},
		"destination_id":     7,
		"destination_prefix": "hubspot_",
		"replication_type":   "HISTORICAL_AND_INCREMENTAL",
		"load_mode":          "MERGE",
		"schema_evolution":   "ALLOW_ALL",
		"schedule":           map[string]any{"sync_type": "SCHEDULED", "frequency_minutes": 60},
		// Hevo stores a threshold for ALL_OR_NONE too; it is not imported.
		"failure_handling_policy": map[string]any{"object_failure_level": "ALL_OR_NONE", "object_failure_threshold": 10000},
		"latency_alert":           map[string]any{"enabled": true, "threshold_minutes": 180},
		"created_at":              1760000000000,
		"updated_at":              1760000000000,
	}

	// The configuration as generated: no source_config, since Hevo never returns it.
	config := strings.Replace(withoutSourceConfig(pipelineConfig("HubSpot prod", "60", "", "MERGE")),
		"  failure_handling_policy = {", "  latency_alert = {\n    enabled           = true\n    threshold_minutes = 180\n  }\n"+
			"  failure_handling_policy = {", 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:             config,
				ResourceName:       "hevo_pipeline.p",
				ImportState:        true,
				ImportStateId:      "42",
				ImportStatePersist: true,
				ImportStateCheck: func(states []*instanceState) error {
					if got := states[0].Attributes["latency_alert.threshold_minutes"]; got != "180" {
						return fmt.Errorf("imported latency_alert.threshold_minutes = %q, want 180", got)
					}

					return nil
				},
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestPipelineResourceImportThenApplyMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := &fakePipelines{}
	fake.register(t, client)

	// A Pipeline that already exists, built in the Hevo UI.
	fake.exists = true
	fake.pipeline = map[string]any{
		"id": 42, "name": "HubSpot prod", "state": "Disabled", "disabled": true,
		"health":                  map[string]any{"value": "HEALTHY"},
		"source":                  map[string]any{"name": "HubSpot", "connector_id": "hubspot", "type": "HUBSPOT"},
		"destination_id":          7,
		"destination_prefix":      "hubspot_",
		"replication_type":        "HISTORICAL_AND_INCREMENTAL",
		"load_mode":               "MERGE",
		"schema_evolution":        "ALLOW_ALL",
		"schedule":                map[string]any{"sync_type": "SCHEDULED", "frequency_minutes": 30},
		"failure_handling_policy": map[string]any{"object_failure_level": "ALL_OR_NONE", "object_failure_threshold": 0},
		"created_at":              1760000000000,
		"updated_at":              1760000000000,
	}

	config := pipelineConfig("HubSpot prod", "30", "pat-2", "MERGE")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:             config,
				ResourceName:       "hevo_pipeline.p",
				ImportState:        true,
				ImportStateId:      "42",
				ImportStatePersist: true,
			},
			{
				// The first apply records source_config without sending it.
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hevo_pipeline.p", "source_config.private_app_access_token", "pat-2"),
					func(*terraformState) error {
						if fake.sourcePatches != 0 {
							return fmt.Errorf("Source PATCH calls = %d, want 0: an imported source_config must not be resent", fake.sourcePatches)
						}

						return nil
					},
				),
			},
			{
				// A later change to it is sent, only the changed key.
				Config: pipelineConfig("HubSpot prod", "30", "pat-3", "MERGE"),
				Check: func(*terraformState) error {
					want := map[string]any{"config": map[string]any{"private_app_access_token": "pat-3"}}
					if !reflect.DeepEqual(fake.sourcePatch, want) {
						return fmt.Errorf("Source PATCH = %v, want %v", fake.sourcePatch, want)
					}

					return nil
				},
			},
		},
	})
}
