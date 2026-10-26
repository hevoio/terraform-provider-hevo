package model

import (
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hevoio/hevo-go/destinations"
	"github.com/hevoio/hevo-go/pipelines"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
)

// DestinationSummaryModel is a Destination without its config.
type DestinationSummaryModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	ConnectorID types.String `tfsdk:"connector_id"`
	Type        types.String `tfsdk:"type"`
	Status      types.String `tfsdk:"status"`
	CreatedAt   types.String `tfsdk:"created_at"`
	CreatedBy   types.String `tfsdk:"created_by"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
	UpdatedBy   types.String `tfsdk:"updated_by"`
}

// NewDestinationSummary maps one API Destination.
func NewDestinationSummary(d destinations.DestinationDetails) DestinationSummaryModel {
	return DestinationSummaryModel{
		ID:          types.StringValue(strconv.FormatInt(d.ID, 10)),
		Name:        types.StringValue(d.Name),
		ConnectorID: types.StringValue(d.ConnectorID),
		Type:        types.StringValue(d.Type),
		Status:      types.StringValue(d.Status),
		CreatedAt:   millisToRFC3339(d.CreatedAt),
		CreatedBy:   types.StringValue(d.CreatedBy),
		UpdatedAt:   millisToRFC3339(d.UpdatedAt),
		UpdatedBy:   types.StringValue(d.UpdatedBy),
	}
}

// DestinationDatasourceModel is the Terraform value of the hevo_destination data source.
type DestinationDatasourceModel struct {
	DestinationSummaryModel

	Config types.Dynamic `tfsdk:"config"`
}

// ReadFromResponse sets every field, config as Hevo returns it.
func (m *DestinationDatasourceModel) ReadFromResponse(d destinations.DestinationDetails) diag.Diagnostics {
	m.DestinationSummaryModel = NewDestinationSummary(d)

	config, diags := core.MapToDynamic(d.Config)
	m.Config = config

	return diags
}

// DestinationsDatasourceModel is the Terraform value of the hevo_destinations data source.
type DestinationsDatasourceModel struct {
	Destinations []DestinationSummaryModel `tfsdk:"destinations"`
}

// PipelineSummaryModel is a Pipeline as the lookup data sources expose it.
type PipelineSummaryModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	State             types.String `tfsdk:"state"`
	Disabled          types.Bool   `tfsdk:"disabled"`
	Health            types.String `tfsdk:"health"`
	SourceName        types.String `tfsdk:"source_name"`
	SourceConnectorID types.String `tfsdk:"source_connector_id"`
	SourceType        types.String `tfsdk:"source_type"`
	DestinationID     types.String `tfsdk:"destination_id"`
	DestinationPrefix types.String `tfsdk:"destination_prefix"`
	ReplicationType   types.String `tfsdk:"replication_type"`
	LoadMode          types.String `tfsdk:"load_mode"`
	SchemaEvolution   types.String `tfsdk:"schema_evolution"`
	SyncType          types.String `tfsdk:"sync_type"`
	FrequencyMinutes  types.Int64  `tfsdk:"frequency_minutes"`
	CreatedAt         types.String `tfsdk:"created_at"`
	CreatedBy         types.String `tfsdk:"created_by"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
	UpdatedBy         types.String `tfsdk:"updated_by"`
}

// NewPipelineSummary maps one API Pipeline.
func NewPipelineSummary(p pipelines.PipelineDetails) PipelineSummaryModel {
	return PipelineSummaryModel{
		ID:                types.StringValue(strconv.FormatInt(p.ID, 10)),
		Name:              types.StringValue(p.Name),
		State:             types.StringValue(p.State),
		Disabled:          types.BoolValue(p.Disabled),
		Health:            types.StringValue(p.Health.Value),
		SourceName:        types.StringValue(p.Source.Name),
		SourceConnectorID: types.StringValue(p.Source.ConnectorID),
		SourceType:        types.StringValue(p.Source.Type),
		DestinationID:     types.StringValue(strconv.FormatInt(p.DestinationID, 10)),
		DestinationPrefix: types.StringValue(p.DestinationPrefix),
		ReplicationType:   types.StringValue(p.ReplicationType),
		LoadMode:          types.StringValue(p.LoadMode),
		SchemaEvolution:   types.StringValue(p.SchemaEvolution),
		SyncType:          types.StringValue(p.Schedule.SyncType),
		FrequencyMinutes:  types.Int64Value(p.Schedule.FrequencyMinutes),
		CreatedAt:         millisToRFC3339(p.CreatedAt),
		CreatedBy:         types.StringValue(p.CreatedBy),
		UpdatedAt:         millisToRFC3339(p.UpdatedAt),
		UpdatedBy:         types.StringValue(p.UpdatedBy),
	}
}

// PipelinesDatasourceModel is the Terraform value of the hevo_pipelines data source.
type PipelinesDatasourceModel struct {
	States            []types.String         `tfsdk:"states"`
	SourceConnectorID types.String           `tfsdk:"source_connector_id"`
	DestinationID     types.String           `tfsdk:"destination_id"`
	Pipelines         []PipelineSummaryModel `tfsdk:"pipelines"`
}
