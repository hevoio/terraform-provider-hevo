// Package model maps each resource's Terraform values to and from the hevo-go types.
package model

import (
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hevoio/hevo-go/destinations"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
)

// DestinationResourceModel is the Terraform value of hevo_destination.
type DestinationResourceModel struct {
	ID          types.String  `tfsdk:"id"`
	Name        types.String  `tfsdk:"name"`
	ConnectorID types.String  `tfsdk:"connector_id"`
	Config      types.Dynamic `tfsdk:"config"`
	ForceDelete types.Bool    `tfsdk:"force_delete"`
	Type        types.String  `tfsdk:"type"`
	Status      types.String  `tfsdk:"status"`
	CreatedAt   types.String  `tfsdk:"created_at"`
	CreatedBy   types.String  `tfsdk:"created_by"`
	UpdatedAt   types.String  `tfsdk:"updated_at"`
	UpdatedBy   types.String  `tfsdk:"updated_by"`
}

// ReadFromResponse sets the computed fields from the API's Destination. It leaves Config and
// ForceDelete alone: the caller decides between the planned config and a projection.
func (m *DestinationResourceModel) ReadFromResponse(d destinations.DestinationDetails) {
	m.ID = types.StringValue(strconv.FormatInt(d.ID, 10))
	m.Name = types.StringValue(d.Name)
	m.ConnectorID = types.StringValue(d.ConnectorID)
	m.Type = types.StringValue(d.Type)
	m.Status = types.StringValue(d.Status)
	m.CreatedAt = millisToRFC3339(d.CreatedAt)
	m.CreatedBy = types.StringValue(d.CreatedBy)
	m.UpdatedAt = millisToRFC3339(d.UpdatedAt)
	m.UpdatedBy = types.StringValue(d.UpdatedBy)
}

// ReadConfigFromResponse sets Config to the API's config projected onto the keys in state, or to
// every non-credential key on import (when state has no config).
func (m *DestinationResourceModel) ReadConfigFromResponse(
	d destinations.DestinationDetails,
	schema *core.Property,
) diag.Diagnostics {
	mask, diags := core.DynamicToMap(m.Config)
	if diags.HasError() {
		return diags
	}

	config, d2 := core.MapToDynamic(core.ProjectConfig(d.Config, mask, schema))
	diags.Append(d2...)

	m.Config = config

	return diags
}

// CopyComputed takes the computed fields from state, for an update that sends nothing to the API.
func (m *DestinationResourceModel) CopyComputed(state DestinationResourceModel) {
	m.ID = state.ID
	m.Type = state.Type
	m.Status = state.Status
	m.CreatedAt = state.CreatedAt
	m.CreatedBy = state.CreatedBy
	m.UpdatedAt = state.UpdatedAt
	m.UpdatedBy = state.UpdatedBy
}

// DestinationID returns the numeric id held in ID.
func (m *DestinationResourceModel) DestinationID() (int64, error) {
	return strconv.ParseInt(m.ID.ValueString(), 10, 64)
}

func millisToRFC3339(ms int64) types.String {
	if ms == 0 {
		return types.StringNull()
	}

	return types.StringValue(time.UnixMilli(ms).UTC().Format(time.RFC3339))
}
