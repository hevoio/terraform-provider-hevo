package resources_test

import (
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	sdkmock "github.com/hevoio/hevo-go/tests/mock"

	"github.com/hevoio/terraform-provider-hevo/hevo/tests/mock"
)

// fakePipelineState is Pipeline 42's enable and disable actions. Each action passes through its
// transitional state for one read.
type fakePipelineState struct {
	mu         sync.Mutex
	disabled   bool
	transition string
	enables    int
	disables   int
	noObjects  bool
}

func (f *fakePipelineState) setDisabled(disabled bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.disabled = disabled
}

func (f *fakePipelineState) body() string {
	state := "Initialized"
	if f.disabled {
		state = "Disabled"
	}

	if f.transition != "" {
		state, f.transition = f.transition, ""
	}

	return fmt.Sprintf(`{"code": "Success", "data": {"id": 42, "state": %q, "disabled": %t}}`, state, f.disabled)
}

func (f *fakePipelineState) register(client *sdkmock.HTTPClient) {
	client.When(http.MethodGet, "/api/v2/pipelines/42").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		return sdkmock.NewResponse(req, http.StatusOK, f.body()), nil
	})

	client.When(http.MethodPost, "/api/v2/pipelines/42/actions/enable").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		if f.noObjects {
			return sdkmock.NewResponse(req, http.StatusBadRequest,
				`{"code": "INVALID_INPUT", "message": "The Pipeline has no selected objects. Select at least one object before enabling it."}`), nil
		}

		f.enables++
		f.disabled, f.transition = false, "Enabling"

		return sdkmock.NewResponse(req, http.StatusAccepted, f.body()), nil
	})

	client.When(http.MethodPost, "/api/v2/pipelines/42/actions/disable").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		f.disables++
		f.disabled, f.transition = true, "Disabling"

		return sdkmock.NewResponse(req, http.StatusAccepted, f.body()), nil
	})
}

func stateConfig(paused bool) string {
	return mock.ProviderConfig + fmt.Sprintf(`
resource "hevo_pipeline_state" "p" {
  pipeline_id = "42"
  paused      = %t
}
`, paused)
}

func TestPipelineStateResourceMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := &fakePipelineState{disabled: true}
	fake.register(client)

	count := func(want, got *int, name string) func(*terraformState) error {
		return func(*terraformState) error {
			if *got != *want {
				return fmt.Errorf("%s calls = %d, want %d", name, *got, *want)
			}

			return nil
		}
	}
	one, two := 1, 2

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: stateConfig(false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hevo_pipeline_state.p", "id", "42"),
					resource.TestCheckResourceAttr("hevo_pipeline_state.p", "paused", "false"),
					count(&one, &fake.enables, "enable"),
				),
			},
			{
				// Paused in the Hevo UI: drift.
				PreConfig:          func() { fake.setDisabled(true) },
				Config:             stateConfig(false),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: stateConfig(false),
				Check:  count(&two, &fake.enables, "enable"),
			},
			{
				Config: stateConfig(true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hevo_pipeline_state.p", "paused", "true"),
					count(&one, &fake.disables, "disable"),
				),
			},
			{
				ResourceName:            "hevo_pipeline_state.p",
				ImportState:             true,
				ImportStateId:           "42",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			{
				// Removing the resource leaves the Pipeline paused.
				Config: mock.ProviderConfig,
				Check: func(*terraformState) error {
					if !fake.disabled || fake.enables != 2 || fake.disables != 1 {
						return fmt.Errorf("removing the resource changed the Pipeline: %+v", fake)
					}

					return nil
				},
			},
		},
	})
}

func TestPipelineStateResourceEnableNeedsObjectsMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	(&fakePipelineState{disabled: true, noObjects: true}).register(client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config:      stateConfig(false),
				ExpectError: regexp.MustCompile(`(?s)Unable to enable the Pipeline.*no selected\s+objects`),
			},
		},
	})
}
