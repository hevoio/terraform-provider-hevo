package model

import (
	"slices"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/types"
	pipelineobjects "github.com/hevoio/hevo-go/pipeline_objects"
)

// SelectableObjectStatuses are the object statuses under which Hevo lets an object be selected or
// deselected. Objects in any other status (INACCESSIBLE, DELETED, DISABLED) cannot change.
var SelectableObjectStatuses = []string{"ACTIVE", "INACTIVE", "RESETTING", "BLOCKED", "INCONSISTENT"}

// PipelineObjectsResourceModel is the Terraform value of hevo_pipeline_objects.
type PipelineObjectsResourceModel struct {
	ID         types.String                   `tfsdk:"id"`
	PipelineID types.String                   `tfsdk:"pipeline_id"`
	Select     types.String                   `tfsdk:"select"`
	Objects    map[string]PipelineObjectModel `tfsdk:"objects"`
	Timeouts   timeouts.Value                 `tfsdk:"timeouts"`
}

// PipelineObjectModel is one object's desired selection.
type PipelineObjectModel struct {
	Enabled        types.Bool   `tfsdk:"enabled"`
	LoadMode       types.String `tfsdk:"load_mode"`
	ExcludedFields types.Set    `tfsdk:"excluded_fields"`
}

// ObjectKey is how an object is named in hevo_pipeline_objects and in a selection: its fully
// qualified name (schema.object), or its name for a source without schemas.
func ObjectKey(o pipelineobjects.PipelineObject) string {
	if o.FullyQualifiedName != "" {
		return o.FullyQualifiedName
	}

	return o.Name
}

// IsSelectable reports whether an object may be selected or deselected, so whether it counts
// toward drift. A child object follows its parent and never counts on its own.
func IsSelectable(o pipelineobjects.PipelineObject) bool {
	return o.ParentObject == "" && slices.Contains(SelectableObjectStatuses, o.Status)
}

// ReadFromObjects sets Objects from the Pipeline's objects. Each object in state takes its remote
// selection, and its load mode when the user set it. Its excluded fields stay as state has them:
// the objects list does not return fields, and a GET per object would not scale. An object outside
// state that departs from the desired set is added, so the plan shows it as drift: under
// "listed" one selected outside Terraform, under "all" one deselected outside Terraform. On
// import (no select in state) the selection is read as "listed", with each object's load mode
// when it differs from the Pipeline's, pipelineLoadMode.
func (m *PipelineObjectsResourceModel) ReadFromObjects(remote []pipelineobjects.PipelineObject, pipelineLoadMode string) {
	imported := m.Select.IsNull() || m.Select.IsUnknown()
	if imported {
		m.Select = types.StringValue("listed")
	}

	listed := m.Select.ValueString() == "listed"
	result := map[string]PipelineObjectModel{}

	for _, o := range remote {
		key := ObjectKey(o)
		prior, inState := m.Objects[key]

		switch {
		case inState:
			current := PipelineObjectModel{Enabled: types.BoolValue(o.Selected), LoadMode: types.StringNull(), ExcludedFields: prior.ExcludedFields}
			if !prior.LoadMode.IsNull() {
				current.LoadMode = types.StringValue(o.LoadMode)
			}

			result[key] = current
		case !IsSelectable(o):
		case listed && o.Selected, !listed && !o.Selected:
			result[key] = outsideObject(o, imported, pipelineLoadMode)
		}
	}

	if len(result) == 0 && m.Objects == nil {
		result = nil
	}

	m.Objects = result
}

// outsideObject is an object found outside state. Its load mode is read only on import, and only
// when it differs from the Pipeline's.
func outsideObject(o pipelineobjects.PipelineObject, imported bool, pipelineLoadMode string) PipelineObjectModel {
	m := PipelineObjectModel{
		Enabled:        types.BoolValue(o.Selected),
		LoadMode:       types.StringNull(),
		ExcludedFields: types.SetNull(types.StringType),
	}
	if imported && o.LoadMode != "" && o.LoadMode != pipelineLoadMode {
		m.LoadMode = types.StringValue(o.LoadMode)
	}

	return m
}
