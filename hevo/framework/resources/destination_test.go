package resources_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	sdkmock "github.com/hevoio/hevo-go/tests/mock"

	"github.com/hevoio/terraform-provider-hevo/hevo/tests/mock"
)

const snowflakeSchema = `{"code": "Success", "data": {"connector_id": "snowflake", "type": "SNOWFLAKE", "display_name": "Snowflake",
  "schema": {"type": "object", "properties": {
    "name": {"type": "string"}, "connector_id": {"const": "snowflake"},
    "config": {"type": "object", "required": ["account_url", "username", "password"], "properties": {
      "account_url": {"type": "string"},
      "warehouse":   {"type": "string"},
      "username":    {"type": "string"},
      "password":    {"type": "string", "format": "password", "writeOnly": true},
      "port":        {"type": "integer"},
      "auth_type":   {"type": "string", "oneOf": [{"const": "PASSWORD"}, {"const": "KEY_PAIR"}]}
    }}}}}}`

// fakeDestinations is an in-memory Destinations API: one Destination, id 7.
type fakeDestinations struct {
	mu               sync.Mutex
	exists           bool
	name             string
	config           map[string]any
	lastPatch        map[string]any
	pipelinesLoading bool
	deleteConflicts  int
	patches          int
}

func (f *fakeDestinations) setConfig(key string, value any) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.config[key] = value
}

// response renders the Destination as the API reads it back: credentials masked, plus a key
// Hevo adds that the user never set.
func (f *fakeDestinations) response(req *http.Request, status int, code string) *http.Response {
	config := maps.Clone(f.config)
	config["password"] = "******"
	config["region"] = "us-east-1"

	data, _ := json.Marshal(map[string]any{
		"id": 7, "name": f.name, "connector_id": "snowflake", "type": "SNOWFLAKE", "status": "CREATED",
		"config": config, "created_at": 1760000000000, "created_by": "ops@acme.com",
		"updated_at": 1760000000000, "updated_by": "ops@acme.com",
	})

	return sdkmock.NewResponse(req, status, fmt.Sprintf(`{"code": %q, "data": %s}`, code, data))
}

func notFound(req *http.Request) *http.Response {
	return sdkmock.NewResponse(req, http.StatusNotFound, `{"code": "RESOURCE_NOT_FOUND", "message": "No Destination exists with id 7."}`)
}

func decode(t *testing.T, req *http.Request) map[string]any {
	t.Helper()

	body, _ := io.ReadAll(req.Body)

	m := map[string]any{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Errorf("decoding %s: %v", body, err)
	}

	return m
}

func (f *fakeDestinations) register(t *testing.T, client *sdkmock.HTTPClient) {
	t.Helper()

	client.When(http.MethodGet, "/api/v2/metadata/destinations/connectors/snowflake").ThenReply(http.StatusOK, snowflakeSchema)

	client.When(http.MethodPost, "/api/v2/destinations").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		body := decode(t, req)
		name, _ := body["name"].(string)
		f.exists, f.name = true, name
		f.config, _ = body["config"].(map[string]any)

		return f.response(req, http.StatusOK, "Success"), nil
	})

	client.When(http.MethodGet, "/api/v2/destinations/7").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		if !f.exists {
			return notFound(req), nil
		}

		return f.response(req, http.StatusOK, "Success"), nil
	})

	client.When(http.MethodPatch, "/api/v2/destinations/7").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		body := decode(t, req)
		f.lastPatch = body
		f.patches++

		if name, ok := body["name"].(string); ok {
			f.name = name
		}

		if config, ok := body["config"].(map[string]any); ok {
			maps.Copy(f.config, config)
		}

		return f.response(req, http.StatusOK, "Success"), nil
	})

	client.When(http.MethodDelete, "/api/v2/destinations/7").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		if !f.exists {
			return notFound(req), nil
		}

		if f.pipelinesLoading && req.URL.Query().Get("confirm_override") != "true" && f.deleteConflicts > 0 {
			f.deleteConflicts--

			return sdkmock.NewResponse(req, http.StatusConflict,
				`{"code": "RESOURCE_CONFLICT", "message": "Pipelines load into this Destination: Orders (42)."}`), nil
		}

		f.exists = false

		return f.response(req, http.StatusAccepted, "Accepted"), nil
	})
}

func destinationConfig(name, warehouse, extra string) string {
	return mock.ProviderConfig + fmt.Sprintf(`
resource "hevo_destination" "sf" {
  name         = %q
  connector_id = "snowflake"
  config = {
    account_url = "acme.snowflakecomputing.com"
    warehouse   = %q
    username    = "HEVO"
    password    = "s3cret"
    port        = 443
  }
  %s
}
`, name, warehouse, extra)
}

func TestDestinationResourceMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := &fakeDestinations{}
	fake.register(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: destinationConfig("Snowflake", "LOAD_WH", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hevo_destination.sf", "id", "7"),
					resource.TestCheckResourceAttr("hevo_destination.sf", "type", "SNOWFLAKE"),
					resource.TestCheckResourceAttr("hevo_destination.sf", "status", "CREATED"),
					resource.TestCheckResourceAttr("hevo_destination.sf", "created_at", "2025-10-09T08:53:20Z"),
					resource.TestCheckResourceAttr("hevo_destination.sf", "config.password", "s3cret"),
					resource.TestCheckResourceAttr("hevo_destination.sf", "force_delete", "false"),
					resource.TestCheckNoResourceAttr("hevo_destination.sf", "config.region"),
					func(*terraformState) error {
						if fake.config["password"] != "s3cret" {
							return fmt.Errorf("create sent password %v", fake.config["password"])
						}

						return nil
					},
				),
			},
			{
				// Rename and change one key: the PATCH carries only those.
				Config: destinationConfig("Snowflake prod", "LOAD_WH_2", ""),
				Check: func(*terraformState) error {
					expected := map[string]any{"name": "Snowflake prod", "config": map[string]any{"warehouse": "LOAD_WH_2"}}
					if fmt.Sprint(fake.lastPatch) != fmt.Sprint(expected) {
						return fmt.Errorf("PATCH body = %v, want %v", fake.lastPatch, expected)
					}

					return nil
				},
			},
			{
				// A key changed outside Terraform shows as drift.
				PreConfig:          func() { fake.setConfig("warehouse", "UI_WH") },
				Config:             destinationConfig("Snowflake prod", "LOAD_WH_2", ""),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				// Applying again puts the configured value back.
				Config: destinationConfig("Snowflake prod", "LOAD_WH_2", ""),
			},
			{
				ResourceName:      "hevo_destination.sf",
				ImportState:       true,
				ImportStateVerify: true,
				// Credentials are never read back, and force_delete lives only in state.
				ImportStateVerifyIgnore: []string{"config", "force_delete"},
			},
		},
	})
}

func TestDestinationResourceImportReadsNonSecretConfigMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := &fakeDestinations{}
	fake.register(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: destinationConfig("Snowflake", "LOAD_WH", "")},
			{
				ResourceName:  "hevo_destination.sf",
				ImportState:   true,
				ImportStateId: "7",
				ImportStateCheck: func(states []*instanceState) error {
					attrs := states[0].Attributes
					if attrs["config.warehouse"] != "LOAD_WH" || attrs["config.region"] != "us-east-1" {
						return fmt.Errorf("imported config = %v", attrs)
					}

					if _, ok := attrs["config.password"]; ok {
						return fmt.Errorf("import read a masked credential: %v", attrs["config.password"])
					}

					return nil
				},
			},
		},
	})
}

func TestDestinationResourceValidatesConfigMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	(&fakeDestinations{}).register(t, client)

	bad := mock.ProviderConfig + `
resource "hevo_destination" "sf" {
  name         = "Snowflake"
  connector_id = "snowflake"
  config = {
    account_url = "acme.snowflakecomputing.com"
    username    = "HEVO"
    auth_type   = "OAUTH"
    port        = "four"
    warehous    = "LOAD_WH"
  }
}
`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: bad, ExpectError: regexp.MustCompile(`Unknown config key`)},
			{Config: bad, ExpectError: regexp.MustCompile(`Missing required config key`)},
			{Config: bad, ExpectError: regexp.MustCompile(`Config value not allowed`)},
			{Config: bad, ExpectError: regexp.MustCompile(`Wrong config value type`)},
		},
	})
}

func TestDestinationResourceForceDeleteMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := &fakeDestinations{pipelinesLoading: true, deleteConflicts: 1}
	fake.register(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: destinationConfig("Snowflake", "LOAD_WH", "")},
			{
				// Without force_delete, a Destination that Pipelines use is not deleted.
				Config:      destinationConfig("Snowflake", "LOAD_WH", ""),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`(?s)Destination still has Pipelines.*Orders\s+\(42\).*force_delete`),
			},
			{
				// Turning force_delete on changes only state.
				Config: destinationConfig("Snowflake", "LOAD_WH", "force_delete = true"),
				Check: func(*terraformState) error {
					if fake.patches != 0 {
						return fmt.Errorf("force_delete alone sent %d PATCH calls", fake.patches)
					}

					return nil
				},
			},
		},
		// The final destroy sends confirm_override and succeeds.
		CheckDestroy: func(*terraformState) error {
			if fake.exists {
				return errors.New("the Destination still exists after destroy")
			}

			return nil
		},
	})
}
