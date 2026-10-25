package model

import (
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hevoio/hevo-go/pipelines"
)

// PipelineResourceModel is the Terraform value of hevo_pipeline.
type PipelineResourceModel struct {
	ID                    types.String                `tfsdk:"id"`
	Name                  types.String                `tfsdk:"name"`
	SourceName            types.String                `tfsdk:"source_name"`
	SourceConnectorID     types.String                `tfsdk:"source_connector_id"`
	SourceConfig          types.Dynamic               `tfsdk:"source_config"`
	SourceAuth            types.Dynamic               `tfsdk:"source_auth"`
	DestinationID         types.String                `tfsdk:"destination_id"`
	DestinationPrefix     types.String                `tfsdk:"destination_prefix"`
	ReplicationType       types.String                `tfsdk:"replication_type"`
	LoadMode              types.String                `tfsdk:"load_mode"`
	SchemaEvolution       types.String                `tfsdk:"schema_evolution"`
	Schedule              *ScheduleModel              `tfsdk:"schedule"`
	FailureHandlingPolicy *FailureHandlingPolicyModel `tfsdk:"failure_handling_policy"`
	LatencyAlert          *LatencyAlertModel          `tfsdk:"latency_alert"`
	SourceType            types.String                `tfsdk:"source_type"`
	State                 types.String                `tfsdk:"state"`
	Disabled              types.Bool                  `tfsdk:"disabled"`
	Health                types.String                `tfsdk:"health"`
	CreatedAt             types.String                `tfsdk:"created_at"`
	CreatedBy             types.String                `tfsdk:"created_by"`
	UpdatedAt             types.String                `tfsdk:"updated_at"`
	UpdatedBy             types.String                `tfsdk:"updated_by"`
	Timeouts              timeouts.Value              `tfsdk:"timeouts"`
}

// ScheduleModel is the Pipeline's schedule.
type ScheduleModel struct {
	SyncType         types.String `tfsdk:"sync_type"`
	FrequencyMinutes types.Int64  `tfsdk:"frequency_minutes"`
}

// FailureHandlingPolicyModel is the Pipeline's failure handling policy.
type FailureHandlingPolicyModel struct {
	ObjectFailureLevel     types.String `tfsdk:"object_failure_level"`
	ObjectFailureThreshold types.Int64  `tfsdk:"object_failure_threshold"`
}

// LatencyAlertModel is the Pipeline's latency alert.
type LatencyAlertModel struct {
	Enabled          types.Bool  `tfsdk:"enabled"`
	ThresholdMinutes types.Int64 `tfsdk:"threshold_minutes"`
}

// PipelineID returns the numeric id held in ID.
func (m *PipelineResourceModel) PipelineID() (int64, error) {
	return strconv.ParseInt(m.ID.ValueString(), 10, 64)
}

// ReadFromResponse sets every field the API returns. The Source's config and auth are never
// returned, so they are left as they are. An optional nested value the user did not set stays
// null, so the values Hevo fills in do not show as drift.
func (m *PipelineResourceModel) ReadFromResponse(p pipelines.PipelineDetails) {
	m.ID = types.StringValue(strconv.FormatInt(p.ID, 10))
	m.Name = types.StringValue(p.Name)
	m.SourceName = types.StringValue(p.Source.Name)
	m.SourceConnectorID = types.StringValue(p.Source.ConnectorID)
	m.SourceType = types.StringValue(p.Source.Type)
	m.DestinationID = types.StringValue(strconv.FormatInt(p.DestinationID, 10))
	m.DestinationPrefix = types.StringValue(p.DestinationPrefix)
	m.ReplicationType = types.StringValue(p.ReplicationType)
	m.LoadMode = types.StringValue(p.LoadMode)
	m.SchemaEvolution = types.StringValue(p.SchemaEvolution)

	m.Schedule = &ScheduleModel{
		SyncType:         types.StringValue(p.Schedule.SyncType),
		FrequencyMinutes: keepNullInt64(m.scheduleFrequency(), p.Schedule.FrequencyMinutes),
	}
	m.FailureHandlingPolicy = &FailureHandlingPolicyModel{
		ObjectFailureLevel:     types.StringValue(p.FailureHandlingPolicy.ObjectFailureLevel),
		ObjectFailureThreshold: keepNullInt64(m.failureThreshold(), p.FailureHandlingPolicy.ObjectFailureThreshold),
	}

	if m.LatencyAlert != nil {
		m.LatencyAlert = &LatencyAlertModel{
			Enabled:          types.BoolValue(p.LatencyAlert.Enabled),
			ThresholdMinutes: keepNullInt64(m.LatencyAlert.ThresholdMinutes, int64(p.LatencyAlert.ThresholdMinutes)),
		}
	}

	m.State = types.StringValue(p.State)
	m.Disabled = types.BoolValue(p.Disabled)
	m.Health = types.StringValue(p.Health.Value)
	m.CreatedAt = millisToRFC3339(p.CreatedAt)
	m.CreatedBy = types.StringValue(p.CreatedBy)
	m.UpdatedAt = millisToRFC3339(p.UpdatedAt)
	m.UpdatedBy = types.StringValue(p.UpdatedBy)
}

func (m *PipelineResourceModel) scheduleFrequency() types.Int64 {
	if m.Schedule == nil {
		return types.Int64Unknown()
	}

	return m.Schedule.FrequencyMinutes
}

func (m *PipelineResourceModel) failureThreshold() types.Int64 {
	if m.FailureHandlingPolicy == nil {
		return types.Int64Unknown()
	}

	return m.FailureHandlingPolicy.ObjectFailureThreshold
}

// keepNullInt64 returns null when the prior value was null (the user left it out), else the
// remote value. With no prior value (on import), an unset remote value (0) stays null too.
func keepNullInt64(prior types.Int64, remote int64) types.Int64 {
	if prior.IsNull() || (prior.IsUnknown() && remote == 0) {
		return types.Int64Null()
	}

	return types.Int64Value(remote)
}
