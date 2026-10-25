package resources_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	sdkmock "github.com/hevoio/hevo-go/tests/mock"

	"github.com/hevoio/terraform-provider-hevo/hevo/tests/mock"
)

type fakeObject struct {
	selected bool
	status   string
	parent   string
	loadMode string
	excluded map[string]bool
}

// fakeObjects is an in-memory objects API for Pipeline 42, applying selection modes as Hevo
// documents them.
type fakeObjects struct {
	mu        sync.Mutex
	objects   map[string]*fakeObject
	lastPatch map[string]any
	patches   int
}

func newFakeObjects() *fakeObjects {
	return &fakeObjects{objects: map[string]*fakeObject{
		"public.orders": {status: "ACTIVE", loadMode: "MERGE", excluded: map[string]bool{}},
		"public.users":  {status: "ACTIVE", loadMode: "MERGE", excluded: map[string]bool{}},
		"public.events": {status: "ACTIVE", loadMode: "MERGE", excluded: map[string]bool{}},
		"public.items":  {status: "ACTIVE", parent: "public.orders", loadMode: "MERGE", excluded: map[string]bool{}},
		"public.legacy": {status: "DELETED", loadMode: "MERGE", excluded: map[string]bool{}},
	}}
}

func (f *fakeObjects) setSelected(name string, selected bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.objects[name].selected = selected
}

func (f *fakeObjects) selectedNames() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	names := []string{}

	for name, o := range f.objects {
		if o.selected && o.parent == "" {
			names = append(names, name)
		}
	}

	sort.Strings(names)

	return names
}

func (f *fakeObjects) register(t *testing.T, client *sdkmock.HTTPClient) {
	t.Helper()

	client.When(http.MethodGet, "/api/v2/pipelines/42").ThenReply(http.StatusOK, `{"code": "Success", "data": {"id": 42, "state": "Disabled"}}`)

	client.When(http.MethodGet, "/api/v2/pipelines/42/objects").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		items := []map[string]any{}

		for name, o := range f.objects {
			fields := []map[string]any{}
			for _, field := range []string{"id", "email", "ssn"} {
				fields = append(fields, map[string]any{"source_name": field, "selectable": field != "id", "selected": !o.excluded[field]})
			}

			items = append(items, map[string]any{
				"name": name, "status": o.status, "selected": o.selected, "parent_object": o.parent,
				"load_mode": o.loadMode, "fields": fields,
			})
		}

		data, _ := json.Marshal(items)

		return sdkmock.NewResponse(req, http.StatusOK, fmt.Sprintf(`{"code": "Success", "has_more": false, "data": %s}`, data)), nil
	})

	client.When(http.MethodPatch, "/api/v2/pipelines/42/objects").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		body := decode(t, req)
		f.lastPatch = body
		f.patches++
		f.applySelection(body)

		return sdkmock.NewResponse(req, http.StatusAccepted, `{"code": "Accepted", "data": {"id": 42, "state": "Disabled"}}`), nil
	})
}

func (f *fakeObjects) applySelection(body map[string]any) {
	mode, _ := body["selection_mode"].(string)

	for _, o := range f.objects {
		if o.status != "ACTIVE" {
			continue
		}

		switch mode {
		case "EXCLUDE_ALL":
			o.selected = false
		case "INCLUDE_ALL":
			o.selected = true
		}
	}

	named, _ := body["objects"].([]any)
	for _, raw := range named {
		entry, _ := raw.(map[string]any)
		name, _ := entry["name"].(string)
		o := f.objects[name]
		o.selected, _ = entry["selected"].(bool)

		if lm, ok := entry["load_mode"].(string); ok {
			o.loadMode = lm
		}

		for _, field := range asStrings(entry["exclude_fields"]) {
			o.excluded[field] = true
		}

		for _, field := range asStrings(entry["include_fields"]) {
			delete(o.excluded, field)
		}
	}

	// A child object follows its parent.
	for _, o := range f.objects {
		if o.parent != "" {
			o.selected = f.objects[o.parent].selected
		}
	}
}

func asStrings(v any) []string {
	raw, _ := v.([]any)
	out := make([]string, 0, len(raw))

	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}

	return out
}

func objectsConfig(selectMode, objects string) string {
	return mock.ProviderConfig + fmt.Sprintf(`
resource "hevo_pipeline_objects" "p" {
  pipeline_id = "42"
  select      = %q
  objects = {
%s
  }
}
`, selectMode, objects)
}

func expectSelected(fake *fakeObjects, want ...string) func(*terraformState) error {
	return func(*terraformState) error {
		if got := fake.selectedNames(); !reflect.DeepEqual(got, want) {
			return fmt.Errorf("selected objects = %v, want %v", got, want)
		}

		return nil
	}
}

func TestPipelineObjectsResourceMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := newFakeObjects()
	fake.register(t, client)

	listed := objectsConfig("listed", `
    "public.orders" = {}
    "public.users"  = { load_mode = "APPEND", excluded_fields = ["ssn"] }
`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				Config: listed,
				Check: resource.ComposeAggregateTestCheckFunc(
					expectSelected(fake, "public.orders", "public.users"),
					resource.TestCheckResourceAttr("hevo_pipeline_objects.p", "objects.public.orders.enabled", "true"),
					func(*terraformState) error {
						want := map[string]any{
							"selection_mode": "EXCLUDE_ALL",
							"objects": []any{
								map[string]any{"name": "public.orders", "selected": true},
								map[string]any{"name": "public.users", "selected": true, "load_mode": "APPEND", "exclude_fields": []any{"ssn"}},
							},
						}
						if !reflect.DeepEqual(fake.lastPatch, want) {
							return fmt.Errorf("PATCH = %v, want %v", fake.lastPatch, want)
						}

						return nil
					},
				),
			},
			{
				// An object selected in the Hevo UI is drift under "listed".
				PreConfig:          func() { fake.setSelected("public.events", true) },
				Config:             listed,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: listed,
				Check:  expectSelected(fake, "public.orders", "public.users"),
			},
			{
				// No longer excluding ssn sends it as included.
				Config: objectsConfig("listed", `
    "public.orders" = {}
    "public.users"  = { load_mode = "APPEND", excluded_fields = [] }
`),
				Check: func(*terraformState) error {
					if fake.objects["public.users"].excluded["ssn"] {
						return fmt.Errorf("ssn is still excluded; PATCH = %v", fake.lastPatch)
					}

					return nil
				},
			},
			{
				// "all" replicates everything selectable except what is disabled.
				Config: objectsConfig("all", `
    "public.events" = { enabled = false }
`),
				Check: expectSelected(fake, "public.orders", "public.users"),
			},
			{
				// An object deselected in the Hevo UI is drift under "all".
				PreConfig: func() { fake.setSelected("public.users", false) },
				Config: objectsConfig("all", `
    "public.events" = { enabled = false }
`),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestPipelineObjectsResourceImportMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := newFakeObjects()
	fake.register(t, client)

	config := objectsConfig("listed", `
    "public.orders" = {}
`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				ResourceName:            "hevo_pipeline_objects.p",
				ImportState:             true,
				ImportStateId:           "42",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			{
				// Removing the resource leaves the selection as it is.
				Config: mock.ProviderConfig,
				Check:  expectSelected(fake, "public.orders"),
			},
		},
	})
}
