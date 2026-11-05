package resources

import (
	"context"
	"maps"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hevoio/hevo-go"
	pipelineobjects "github.com/hevoio/hevo-go/pipeline_objects"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core/model"
	hevoschema "github.com/hevoio/terraform-provider-hevo/hevo/framework/core/schema"
)

const objectsPageLimit = 500

// PipelineObjects is the hevo_pipeline_objects resource.
func PipelineObjects() resource.Resource {
	return &pipelineObjects{}
}

type pipelineObjects struct {
	core.ProviderResource
}

var (
	_ resource.ResourceWithConfigure   = (*pipelineObjects)(nil)
	_ resource.ResourceWithImportState = (*pipelineObjects)(nil)
	_ resource.ResourceWithModifyPlan  = (*pipelineObjects)(nil)
)

// ModifyPlan checks that each object an update adds exists in the Pipeline, one GET per added
// object, and fails the plan with Hevo's message for one that does not. Objects already in state
// are not read again. A create is not checked: a new Pipeline lists no objects until Hevo has
// discovered them, which the create waits for.
func (r *pipelineObjects) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() || req.State.Raw.IsNull() || r.GetClient() == nil || r.GetSkipPlanTimeValidation() {
		return
	}

	var plan, state model.PipelineObjectsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() || plan.PipelineID.IsUnknown() {
		return
	}

	id, err := strconv.ParseInt(plan.PipelineID.ValueString(), 10, 64)
	if err != nil {
		return
	}

	added := make([]string, 0, len(plan.Objects))

	for name := range plan.Objects {
		if _, ok := state.Objects[name]; !ok {
			added = append(added, name)
		}
	}

	sort.Strings(added)

	for _, name := range added {
		_, err := r.GetClient().GetPipelineObject().PipelineID(id).ObjectName(name).Do(ctx)
		if hevo.IsNotFound(err) {
			resp.Diagnostics.AddAttributeError(path.Root("objects").AtMapKey(name), "Unknown object", core.ErrorDetail(err))
		}
		// Any other failure is left to the apply.
	}
}

func (r *pipelineObjects) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pipeline_objects"
}

func (r *pipelineObjects) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = hevoschema.PipelineObjectsResourceSchema(ctx)
}

func (r *pipelineObjects) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importByPipelineID(ctx, req, resp)
}

func (r *pipelineObjects) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model.PipelineObjectsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Create(ctx, defaultPipelineTimeout)
	resp.Diagnostics.Append(diags...)

	r.apply(ctx, timeout, plan, nil, &resp.Diagnostics)

	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = plan.PipelineID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pipelineObjects) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state model.PipelineObjectsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	timeout, diags := plan.Timeouts.Update(ctx, defaultPipelineTimeout)
	resp.Diagnostics.Append(diags...)

	r.apply(ctx, timeout, plan, state.Objects, &resp.Diagnostics)

	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = plan.PipelineID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// apply sends the desired selection in one request, retrying while the Pipeline is busy, then
// waits for Hevo to apply it.
func (r *pipelineObjects) apply(
	ctx context.Context,
	timeout time.Duration,
	plan model.PipelineObjectsResourceModel,
	prior map[string]model.PipelineObjectModel,
	diags *diag.Diagnostics,
) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(diags)

		return
	}

	id, err := strconv.ParseInt(plan.PipelineID.ValueString(), 10, 64)
	if err != nil {
		diags.AddAttributeError(path.Root("pipeline_id"), "Invalid pipeline_id", core.ErrorDetail(err))

		return
	}

	call := r.GetClient().UpdatePipelineObjects().PipelineID(id)
	for _, selection := range selections(ctx, plan, prior, diags) {
		call.Object(selection)
	}

	if plan.Select.ValueString() == hevoschema.SelectAll {
		call.SelectionMode(pipelineobjects.SelectionModeIncludeAll)
	} else {
		call.SelectionMode(pipelineobjects.SelectionModeExcludeAll)
	}

	if diags.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err = core.RetryOnConflict(ctx, func(ctx context.Context) error {
		_, err := call.Do(ctx)

		return err
	})
	if err != nil {
		diags.AddError("Unable to update the Pipeline's objects", core.ErrorDetail(err))

		return
	}

	if _, err := core.WaitForPipeline(ctx, r.GetClient(), id); err != nil {
		diags.AddError("The object selection did not finish applying", core.ErrorDetail(err))

		return
	}

	// On a disabled Pipeline the selection lands a little after the request returns, with no
	// transitional state to wait on, so wait until the objects show it. Each poll lists the
	// selectable objects instead of reading every named one, to stay within the API's rate limits.
	err = core.WaitUntil(ctx, "the object selection to apply", func(ctx context.Context) (bool, error) {
		objects, err := r.readObjects(ctx, id, plan.Objects)
		if err != nil {
			return false, err
		}

		return selectionApplied(plan, objects), nil
	})
	if err != nil {
		diags.AddError("The object selection did not finish applying", core.ErrorDetail(err))
	}
}

// selectionApplied reports whether every object the plan names has the selection it asks for.
// Objects the plan does not name are left to the selection mode and not checked, since Hevo
// skips those it cannot select.
func selectionApplied(plan model.PipelineObjectsResourceModel, objects []pipelineobjects.PipelineObject) bool {
	selected := make(map[string]bool, len(objects))
	for _, o := range objects {
		selected[model.ObjectKey(o)] = o.Selected
	}

	for name, object := range plan.Objects {
		want := object.Enabled.IsNull() || object.Enabled.ValueBool()
		if got, ok := selected[name]; ok && got != want {
			return false
		}
	}

	return true
}

// selections builds one entry per object to send. Under "listed" (EXCLUDE_ALL) every enabled
// object is named, so the request leaves exactly those selected; under "all" (INCLUDE_ALL) the
// disabled objects are named, plus enabled ones that carry a load mode or excluded fields, or
// that had fields excluded before. Fields excluded before and no longer excluded are sent as
// included, including for an object removed from the configuration under "all".
func selections(
	ctx context.Context,
	plan model.PipelineObjectsResourceModel,
	prior map[string]model.PipelineObjectModel,
	diags *diag.Diagnostics,
) []*pipelineobjects.ObjectSelection {
	listed := plan.Select.ValueString() == hevoschema.SelectListed
	objects := make(map[string]model.PipelineObjectModel, len(plan.Objects))
	maps.Copy(objects, plan.Objects)

	if !listed {
		addUnnamedWithExcludedFields(objects, prior)
	}

	names := slices.Sorted(maps.Keys(objects))
	result := []*pipelineobjects.ObjectSelection{}

	for _, name := range names {
		object := objects[name]
		enabled := object.Enabled.IsNull() || object.Enabled.ValueBool()
		hasOverrides := !object.LoadMode.IsNull() || !object.ExcludedFields.IsNull() || hasExcludedFields(prior[name])

		if listed && !enabled || !listed && enabled && !hasOverrides {
			continue
		}

		selection := hevo.NewObjectSelection().Name(name).Selected(enabled)
		if enabled {
			addOverrides(ctx, selection, object, prior[name], diags)
		}

		result = append(result, selection)
	}

	return result
}

// addUnnamedWithExcludedFields adds, with default settings, each object in state that excluded
// fields and is no longer named. Under "all" such an object is selected with its defaults, so the
// fields it excluded are included again.
func addUnnamedWithExcludedFields(objects, prior map[string]model.PipelineObjectModel) {
	for name, object := range prior {
		if _, named := objects[name]; !named && hasExcludedFields(object) {
			objects[name] = model.PipelineObjectModel{
				Enabled:        types.BoolNull(),
				LoadMode:       types.StringNull(),
				ExcludedFields: types.SetNull(types.StringType),
			}
		}
	}
}

// hasExcludedFields reports whether an object in state excludes any field.
func hasExcludedFields(object model.PipelineObjectModel) bool {
	return !object.ExcludedFields.IsNull() && !object.ExcludedFields.IsUnknown() &&
		len(object.ExcludedFields.Elements()) > 0
}

func addOverrides(
	ctx context.Context,
	selection *pipelineobjects.ObjectSelection,
	object, prior model.PipelineObjectModel,
	diags *diag.Diagnostics,
) {
	if !object.LoadMode.IsNull() {
		selection.LoadMode(object.LoadMode.ValueString())
	}

	excluded := stringSet(ctx, object.ExcludedFields, diags)
	if len(excluded) > 0 {
		selection.ExcludeFields(excluded...)
	}

	var included []string

	for _, field := range stringSet(ctx, prior.ExcludedFields, diags) {
		if !contains(excluded, field) {
			included = append(included, field)
		}
	}

	if len(included) > 0 {
		selection.IncludeFields(included...)
	}
}

func (r *pipelineObjects) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	var state model.PipelineObjectsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	id, err := strconv.ParseInt(state.PipelineID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid pipeline_id in state", core.ErrorDetail(err))
	}

	if resp.Diagnostics.HasError() {
		return
	}

	objects, err := r.readObjects(ctx, id, state.Objects)
	if hevo.IsNotFound(err) {
		// The Pipeline is gone, and its objects with it.
		resp.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		resp.Diagnostics.AddError("Unable to read the Pipeline's objects", core.ErrorDetail(err))

		return
	}

	// On import, an object's load mode is read when it differs from the Pipeline's.
	pipelineLoadMode := ""

	if state.Select.IsNull() {
		pipeline, err := r.GetClient().GetPipeline().PipelineID(id).Do(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Unable to read the Pipeline", core.ErrorDetail(err))

			return
		}

		pipelineLoadMode = pipeline.Data.LoadMode
	}

	state.ReadFromObjects(objects, pipelineLoadMode)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// readObjects returns what Read needs: the objects state names, plus every object that can still be
// selected or deselected, so one changed outside Terraform shows as drift. It never lists a
// source's other objects, which can number in the tens of thousands.
func (r *pipelineObjects) readObjects(
	ctx context.Context,
	pipelineID int64,
	named map[string]model.PipelineObjectModel,
) ([]pipelineobjects.PipelineObject, error) {
	selectable, err := r.selectableObjects(ctx, pipelineID)
	if err != nil {
		return nil, err
	}

	byKey := make(map[string]pipelineobjects.PipelineObject, len(selectable)+len(named))
	for _, o := range selectable {
		byKey[model.ObjectKey(o)] = o
	}

	missing := map[string]model.PipelineObjectModel{}

	for name, object := range named {
		if _, ok := byKey[name]; !ok {
			missing[name] = object
		}
	}

	fetched, err := r.namedObjects(ctx, pipelineID, missing)
	if err != nil {
		return nil, err
	}

	for _, o := range fetched {
		byKey[model.ObjectKey(o)] = o
	}

	all := make([]pipelineobjects.PipelineObject, 0, len(byKey))
	for _, o := range byKey {
		all = append(all, o)
	}

	return all, nil
}

// namedObjects reads each named object by its fully qualified name; one Hevo no longer has is left
// out, so it shows as drift.
func (r *pipelineObjects) namedObjects(
	ctx context.Context,
	pipelineID int64,
	named map[string]model.PipelineObjectModel,
) ([]pipelineobjects.PipelineObject, error) {
	objects := make([]pipelineobjects.PipelineObject, 0, len(named))

	for name := range named {
		resp, err := r.GetClient().GetPipelineObject().PipelineID(pipelineID).ObjectName(name).Do(ctx)
		if hevo.IsNotFound(err) {
			continue
		}

		if err != nil {
			return nil, err
		}

		objects = append(objects, resp.Data)
	}

	return objects, nil
}

// selectableObjects lists the objects whose selection can change, filtered by status on the
// server.
func (r *pipelineObjects) selectableObjects(ctx context.Context, pipelineID int64) ([]pipelineobjects.PipelineObject, error) {
	all := []pipelineobjects.PipelineObject{}
	cursor := ""

	for {
		call := r.GetClient().ListPipelineObjects().
			PipelineID(pipelineID).
			Limit(objectsPageLimit).
			Statuses(model.SelectableObjectStatuses...)
		if cursor != "" {
			call.Cursor(cursor)
		}

		page, err := call.Do(ctx)
		if err != nil {
			return nil, err
		}

		all = append(all, page.Data...)

		if !page.HasMore || page.NextCursor == "" {
			return all, nil
		}

		cursor = page.NextCursor
	}
}

// Delete leaves the selection as it is, like removing any other part of a Pipeline's
// configuration from Terraform.
func (r *pipelineObjects) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {}

func stringSet(ctx context.Context, set types.Set, diags *diag.Diagnostics) []string {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}

	var values []string

	diags.Append(set.ElementsAs(ctx, &values, false)...)
	sort.Strings(values)

	return values
}

func contains(values []string, v string) bool {
	return slices.Contains(values, v)
}
