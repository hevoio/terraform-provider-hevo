package schema

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

const replacesPipeline = " Changing it replaces the Pipeline, and the new Pipeline loads all historical data again."

// PipelineResourceSchema is the schema of hevo_pipeline.
func PipelineResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Description: "A Pipeline: replicates a Source, created with it, into a Destination. A new Pipeline is " +
			"created disabled; select its objects with `hevo_pipeline_objects` and enable it with " +
			"`hevo_pipeline_state`. Edits run asynchronously in Hevo; the provider waits for each to be applied.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The Pipeline's id.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The Pipeline's display name, unique within the team.",
			},
			"source_name": schema.StringAttribute{
				Required:    true,
				Description: "The Source's display name, unique within the team.",
			},
			"source_connector_id": schema.StringAttribute{
				Required: true,
				Description: "The source connector, for example `hubspot`. List them with the `hevo_connectors` data " +
					"source." + replacesPipeline,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"source_config": schema.DynamicAttribute{
				Required: true,
				Description: "The Source's settings, as the connector's schema in `hevo_connector_metadata` names " +
					"them. Hevo does not return a Source's settings, so changes made outside Terraform are not " +
					"detected; a change here is sent as is, and Hevo tests the connection.",
			},
			"source_auth": schema.DynamicAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "For a Source that authenticates with your own OAuth app: `client_access` " +
					"(`client_id`, `client_secret` and any app-level credential) and `refresh_token` or " +
					"`access_token`. Never read back.",
			},
			"destination_id": schema.StringAttribute{
				Required:      true,
				Description:   "The id of the Destination to load into, such as `hevo_destination.x.id`." + replacesPipeline,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"destination_prefix": schema.StringAttribute{
				Required:      true,
				Description:   "The prefix of the tables the Pipeline creates in the Destination." + replacesPipeline,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"replication_type": schema.StringAttribute{
				Required:      true,
				Description:   "`HISTORICAL_AND_INCREMENTAL` or `INCREMENTAL_ONLY`." + replacesPipeline,
				Validators:    []validator.String{stringvalidator.OneOf("HISTORICAL_AND_INCREMENTAL", "INCREMENTAL_ONLY")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"load_mode": schema.StringAttribute{
				Required:    true,
				Description: "How objects load: `MERGE`, `APPEND` or `HISTORY_TRACKING`.",
				Validators:  []validator.String{stringvalidator.OneOf("MERGE", "APPEND", "HISTORY_TRACKING")},
			},
			"schema_evolution": schema.StringAttribute{
				Required:    true,
				Description: "`ALLOW_ALL` replicates new objects and fields as they appear; `BLOCK_ALL` does not.",
				Validators:  []validator.String{stringvalidator.OneOf("ALLOW_ALL", "BLOCK_ALL")},
			},
			"schedule":                pipelineScheduleAttribute(),
			"failure_handling_policy": pipelineFailureHandlingPolicyAttribute(),
			"latency_alert":           pipelineLatencyAlertAttribute(),
			"source_type": schema.StringAttribute{
				Computed:      true,
				Description:   "The source type, for example `HUBSPOT`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"state": schema.StringAttribute{
				Computed:    true,
				Description: "The Pipeline's state as the Hevo UI shows it, for example `Disabled` or `Initialized`.",
			},
			"disabled": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the Pipeline is paused. Change it with `hevo_pipeline_state`.",
			},
			"health": schema.StringAttribute{
				Computed:    true,
				Description: "The Pipeline's health, for example `HEALTHY` or `NEEDS_ATTENTION`.",
			},
			"created_at": schema.StringAttribute{
				Computed:      true,
				Description:   "When the Pipeline was created, in RFC 3339.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_by": schema.StringAttribute{
				Computed:      true,
				Description:   "Who created the Pipeline.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "When the Pipeline was last changed, in RFC 3339.",
			},
			"updated_by": schema.StringAttribute{
				Computed:    true,
				Description: "Who last changed the Pipeline.",
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func pipelineScheduleAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required:    true,
		Description: "When the Pipeline ingests.",
		Attributes: map[string]schema.Attribute{
			"sync_type": schema.StringAttribute{
				Required:    true,
				Description: "`SCHEDULED` or `ON_DEMAND`.",
				Validators:  []validator.String{stringvalidator.OneOf("SCHEDULED", "ON_DEMAND")},
			},
			"frequency_minutes": schema.Int64Attribute{
				Optional:    true,
				Description: "How often a `SCHEDULED` Pipeline ingests.",
			},
		},
	}
}

func pipelineFailureHandlingPolicyAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required:    true,
		Description: "How many objects may fail before a load fails.",
		Attributes: map[string]schema.Attribute{
			"object_failure_level": schema.StringAttribute{
				Required:    true,
				Description: "`PARTIAL` or `ALL_OR_NONE`.",
				Validators:  []validator.String{stringvalidator.OneOf("PARTIAL", "ALL_OR_NONE")},
			},
			"object_failure_threshold": schema.Int64Attribute{
				Optional:    true,
				Description: "How many objects may fail under `PARTIAL`.",
			},
		},
	}
}

func pipelineLatencyAlertAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:    true,
		Description: "Alert when replication falls behind.",
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the alert is on.",
			},
			"threshold_minutes": schema.Int64Attribute{
				Optional:    true,
				Description: "The latency that raises the alert.",
			},
		},
	}
}
