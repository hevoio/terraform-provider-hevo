package schema

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func computedInt64(description string) schema.Int64Attribute {
	return schema.Int64Attribute{Computed: true, Description: description}
}

// PipelineJobsDatasourceSchema is the schema of the hevo_pipeline_jobs data source.
func PipelineJobsDatasourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Reads a Pipeline's jobs, newest first: each ingestion, historical load or resync, with " +
			"its event and object counts. Set `job_id` to read one job with what it did for each object.",
		Attributes: map[string]schema.Attribute{
			"pipeline_id": schema.StringAttribute{
				Required:    true,
				Description: "The Pipeline, such as `hevo_pipeline.x.id`.",
			},
			"job_id": schema.StringAttribute{
				Optional:    true,
				Description: "Read only this job, and fill `objects`.",
			},
			"statuses": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Keep only jobs in these statuses, for example `[\"IN_PROGRESS\"]`.",
			},
			"types": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Keep only jobs of these types, for example `[\"HISTORICAL\"]`.",
			},
			"limit": schema.Int64Attribute{
				Optional:    true,
				Description: "How many of the newest jobs to read, from 1 to 1000. Defaults to 20.",
				Validators:  []validator.Int64{int64validator.Between(1, 1000)},
			},
			"jobs": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The jobs, newest first.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":              computedString("The job's id."),
						"type":            computedString("`HISTORICAL`, `INCREMENTAL`, `REFRESHER`, `RESYNC` or `RESYNC_WITH_DROP_AND_LOAD`."),
						"status":          computedString("The job's status, for example `COMPLETED` or `IN_PROGRESS`."),
						"created_at":      computedString("When the job started, in RFC 3339."),
						"updated_at":      computedString("When the job last changed, in RFC 3339."),
						"events_ingested": computedInt64("Events read from the Source."),
						"events_loaded":   computedInt64("Events loaded into the Destination."),
						"events_failed":   computedInt64("Events that failed."),
						"objects_success": computedInt64("Objects that completed."),
						"objects_queued":  computedInt64("Objects still queued."),
						"objects_skipped": computedInt64("Objects skipped."),
						"objects_failed":  computedInt64("Objects that failed."),
						"duration_ms":     computedInt64("How long the job ran, in milliseconds."),
						"latency_mean_ms": computedInt64("Mean replication latency, in milliseconds."),
						"latency_max_ms":  computedInt64("Maximum replication latency, in milliseconds."),
					},
				},
			},
			"objects": schema.ListNestedAttribute{
				Computed:    true,
				Description: "With `job_id`, what the job did for each object; empty otherwise.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"object_name":     computedString("The object."),
						"status":          computedString("The job's status for this object."),
						"events_ingested": computedInt64("Events read for this object."),
						"events_loaded":   computedInt64("Events loaded for this object."),
						"events_failed":   computedInt64("Events that failed for this object."),
						"offset":          computedString("Where the job stopped reading this object."),
					},
				},
			},
		},
	}
}
