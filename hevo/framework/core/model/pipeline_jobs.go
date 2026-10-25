package model

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hevoio/hevo-go/jobs"
)

// PipelineJobsDatasourceModel is the Terraform value of the hevo_pipeline_jobs data source.
type PipelineJobsDatasourceModel struct {
	PipelineID types.String     `tfsdk:"pipeline_id"`
	JobID      types.String     `tfsdk:"job_id"`
	Statuses   []types.String   `tfsdk:"statuses"`
	Types      []types.String   `tfsdk:"types"`
	Limit      types.Int64      `tfsdk:"limit"`
	Jobs       []JobModel       `tfsdk:"jobs"`
	Objects    []JobObjectModel `tfsdk:"objects"`
}

// JobModel is one job.
type JobModel struct {
	ID             types.String `tfsdk:"id"`
	Type           types.String `tfsdk:"type"`
	Status         types.String `tfsdk:"status"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	EventsIngested types.Int64  `tfsdk:"events_ingested"`
	EventsLoaded   types.Int64  `tfsdk:"events_loaded"`
	EventsFailed   types.Int64  `tfsdk:"events_failed"`
	ObjectsSuccess types.Int64  `tfsdk:"objects_success"`
	ObjectsQueued  types.Int64  `tfsdk:"objects_queued"`
	ObjectsSkipped types.Int64  `tfsdk:"objects_skipped"`
	ObjectsFailed  types.Int64  `tfsdk:"objects_failed"`
	DurationMs     types.Int64  `tfsdk:"duration_ms"`
	LatencyMeanMs  types.Int64  `tfsdk:"latency_mean_ms"`
	LatencyMaxMs   types.Int64  `tfsdk:"latency_max_ms"`
}

// JobObjectModel is what one job did for one object.
type JobObjectModel struct {
	ObjectName     types.String `tfsdk:"object_name"`
	Status         types.String `tfsdk:"status"`
	EventsIngested types.Int64  `tfsdk:"events_ingested"`
	EventsLoaded   types.Int64  `tfsdk:"events_loaded"`
	EventsFailed   types.Int64  `tfsdk:"events_failed"`
	Offset         types.String `tfsdk:"offset"`
}

// NewJob maps one API job.
func NewJob(j jobs.Job) JobModel {
	return JobModel{
		ID:             types.StringValue(j.ID),
		Type:           types.StringValue(j.Type),
		Status:         types.StringValue(j.Status),
		CreatedAt:      millisToRFC3339(j.CreatedAt),
		UpdatedAt:      millisToRFC3339(j.UpdatedAt),
		EventsIngested: types.Int64Value(j.EventsIngested),
		EventsLoaded:   types.Int64Value(j.EventsLoaded),
		EventsFailed:   types.Int64Value(j.EventsFailed),
		ObjectsSuccess: types.Int64Value(j.ObjectsSuccess),
		ObjectsQueued:  types.Int64Value(j.ObjectsQueued),
		ObjectsSkipped: types.Int64Value(j.ObjectsSkipped),
		ObjectsFailed:  types.Int64Value(j.ObjectsFailed),
		DurationMs:     types.Int64Value(j.Duration),
		LatencyMeanMs:  types.Int64Value(j.Latency.Mean),
		LatencyMaxMs:   types.Int64Value(j.Latency.Max),
	}
}

// NewJobObject maps one API job object.
func NewJobObject(o jobs.JobObject) JobObjectModel {
	return JobObjectModel{
		ObjectName:     types.StringValue(o.ObjectName),
		Status:         types.StringValue(o.Status),
		EventsIngested: types.Int64PointerValue(o.EventsIngested),
		EventsLoaded:   types.Int64PointerValue(o.EventsLoaded),
		EventsFailed:   types.Int64PointerValue(o.EventsFailed),
		Offset:         types.StringPointerValue(o.Offset),
	}
}
