package resources_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
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
	// lagReads delays a selection: it shows on the objects only after this many reads.
	lagReads int
	pending  map[string]any
	reads    int
	lists    int
	gets     int
}

func newFakeObjects() *fakeObjects {
	return &fakeObjects{objects: map[string]*fakeObject{
		"public.orders":  {status: "ACTIVE", loadMode: "MERGE", excluded: map[string]bool{}},
		"public.users":   {status: "ACTIVE", loadMode: "MERGE", excluded: map[string]bool{}},
		"public.events":  {status: "ACTIVE", loadMode: "MERGE", excluded: map[string]bool{}},
		"public.items":   {status: "ACTIVE", parent: "public.orders", loadMode: "MERGE", excluded: map[string]bool{}},
		"public.legacy":  {status: "DELETED", loadMode: "MERGE", excluded: map[string]bool{}},
		"public.archive": {status: "INACCESSIBLE", loadMode: "MERGE", excluded: map[string]bool{}},
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

	client.When(http.MethodGet, "/api/v2/pipelines/42").ThenReply(http.StatusOK, `{"code": "Success", "data": {"id": 42, "state": "Disabled", "load_mode": "MERGE"}}`)

	client.When(http.MethodGet, "/api/v2/pipelines/42/objects").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		f.tick()
		f.lists++

		wanted := req.URL.Query()["status"]
		if len(wanted) == 0 {
			// A source can have tens of thousands of objects; the provider must never list them all.
			t.Errorf("objects listed without a status filter: %s", req.URL.RawQuery)
		}

		items := []map[string]any{}

		for name, o := range f.objects {
			// The list, unlike a GET, does not return an object's fields.
			item := f.item(name, o)
			delete(item, "fields")

			status, _ := item["status"].(string)
			if len(wanted) == 0 || slices.Contains(wanted, status) {
				items = append(items, item)
			}
		}

		data, _ := json.Marshal(items)

		return sdkmock.NewResponse(req, http.StatusOK, fmt.Sprintf(`{"code": "Success", "has_more": false, "data": %s}`, data)), nil
	})

	for name := range f.objects {
		client.When(http.MethodGet, "/api/v2/pipelines/42/objects/"+name).ThenCall(func(req *http.Request) (*http.Response, error) {
			f.mu.Lock()
			defer f.mu.Unlock()

			f.tick()
			f.gets++

			data, _ := json.Marshal(f.item(name, f.objects[name]))

			return sdkmock.NewResponse(req, http.StatusOK, fmt.Sprintf(`{"code": "Success", "data": %s}`, data)), nil
		})
	}

	client.When(http.MethodGet, "/api/v2/pipelines/42/objects/public.ordrs").ThenReply(http.StatusNotFound,
		`{"code": "RESOURCE_NOT_FOUND", "message": "No object named 'public.ordrs' was found in Pipeline 42."}`)

	client.When(http.MethodPatch, "/api/v2/pipelines/42/objects").ThenCall(func(req *http.Request) (*http.Response, error) {
		f.mu.Lock()
		defer f.mu.Unlock()

		body := decode(t, req)
		f.lastPatch = body
		f.patches++

		if f.lagReads > 0 {
			f.pending = body
		} else {
			f.applySelection(body)
		}

		return sdkmock.NewResponse(req, http.StatusAccepted, `{"code": "Accepted", "data": {"id": 42, "state": "Disabled"}}`), nil
	})
}

// tick counts a read and, once enough reads have passed, applies a delayed selection.
func (f *fakeObjects) tick() {
	f.reads++
	if f.pending != nil {
		if f.lagReads > 0 {
			f.lagReads--
		} else {
			f.applySelection(f.pending)
			f.pending = nil
		}
	}
}

// item renders an object as the API returns it: a selectable object reports ACTIVE when selected
// and INACTIVE when not.
func (f *fakeObjects) item(name string, o *fakeObject) map[string]any {
	status := o.status
	if status == "ACTIVE" && !o.selected {
		status = "INACTIVE"
	}

	fields := []map[string]any{}
	for _, field := range []string{"id", "email", "ssn"} {
		fields = append(fields, map[string]any{"source_name": field, "selectable": field != "id", "selected": !o.excluded[field]})
	}

	return map[string]any{
		"fully_qualified_name": name, "name": name[strings.LastIndex(name, ".")+1:], "status": status,
		"selected": o.selected, "parent_object": o.parent, "load_mode": o.loadMode, "fields": fields,
	}
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

func expectIncluded(fake *fakeObjects, object, field string) func(*terraformState) error {
	return func(*terraformState) error {
		if fake.objects[object].excluded[field] {
			return fmt.Errorf("%s is still excluded from %s; PATCH = %v", field, object, fake.lastPatch)
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
				// Excluded fields are kept from state, so a field excluded in the Hevo UI is not drift.
				PreConfig: func() { fake.objects["public.users"].excluded["email"] = true },
				Config:    listed,
				PlanOnly:  true,
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
				Check: expectIncluded(fake, "public.users", "ssn"),
			},
			{
				Config: listed,
				Check: func(*terraformState) error {
					if !fake.objects["public.users"].excluded["ssn"] {
						return fmt.Errorf("ssn is not excluded; PATCH = %v", fake.lastPatch)
					}

					return nil
				},
			},
			{
				// Removing excluded_fields includes the fields it excluded.
				Config: objectsConfig("listed", `
    "public.orders" = {}
    "public.users"  = { load_mode = "APPEND" }
`),
				Check: expectIncluded(fake, "public.users", "ssn"),
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

	// An object's load mode is imported only when it differs from the Pipeline's (MERGE).
	config := objectsConfig("listed", `
    "public.orders" = {}
    "public.users"  = { load_mode = "APPEND" }
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
				Check:  expectSelected(fake, "public.orders", "public.users"),
			},
		},
	})
}

func TestPipelineObjectsResourceRejectsUnknownObjectMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := newFakeObjects()
	fake.register(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				// A create is not checked: a new Pipeline lists no objects until Hevo discovers them.
				PreConfig: func() { fake.gets = 0 },
				Config: objectsConfig("listed", `
    "public.orders" = {}
`),
				Check: func(*terraformState) error {
					if fake.gets != 0 {
						return fmt.Errorf("objects read one by one %d times, want 0", fake.gets)
					}

					return nil
				},
			},
			{
				// An object an update adds is checked.
				Config: objectsConfig("listed", `
    "public.orders" = {}
    "public.ordrs"  = {}
`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`No\s+object\s+named\s+'public.ordrs'\s+was\s+found`),
			},
			{
				// An object already in state is not read again at plan time.
				PreConfig: func() { fake.gets = 0 },
				Config: objectsConfig("listed", `
    "public.orders" = { load_mode = "APPEND" }
`),
				Check: func(*terraformState) error {
					if fake.gets != 0 {
						return fmt.Errorf("objects read one by one %d times, want 0", fake.gets)
					}

					return nil
				},
			},
		},
	})
}

func TestPipelineObjectsResourceWaitsForTheSelectionMock(t *testing.T) {
	t.Parallel()

	client, factories := mock.NewProvider()
	fake := newFakeObjects()
	fake.lagReads = 2
	fake.register(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{
				// The selection shows only on the third read; create must not return before it.
				Config: objectsConfig("listed", `
    "public.orders" = {}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					expectSelected(fake, "public.orders"),
					func(*terraformState) error {
						if fake.reads < 3 {
							return fmt.Errorf("objects read %d times, want at least 3", fake.reads)
						}

						// Every poll lists the objects; none reads them one by one.
						if fake.gets != 0 {
							return fmt.Errorf("objects read one by one %d times, want 0", fake.gets)
						}

						return nil
					},
				),
			},
		},
	})
}
