package schema

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
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
			"`hevo_pipeline_state`. Edits run asynchronously in Hevo; the provider waits for each to be applied. " +
			"The Pipeline's state, health and last change move as it runs and as `hevo_pipeline_state` " +
			"enables it, so they are not part of this resource: read them with the `hevo_pipeline` data source.",
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
				Description: "The source connector, for example `SOURCE_CONNECTOR_POSTGRES`. List them with the `hevo_connectors` data " +
					"source." + replacesPipeline,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"source_config": schema.DynamicAttribute{
				Optional: true,
				Description: "The Source's settings, as the connector's schema in `hevo_connector_metadata` names " +
					"them. Required to create a Pipeline, unless `source_auth` alone authorizes the Source. Hevo " +
					"does not return a Source's settings, so changes made outside Terraform are not detected; a " +
					"change here is sent as is, and Hevo tests the connection. After an import it may be left " +
					"out, and the Source keeps its settings; when set, the first apply records it without sending " +
					"it. Removing it later sends nothing. It is not marked sensitive, so that plans show " +
					"which settings change: a credential written as a literal value is shown in plain text. Pass " +
					"credentials through a variable declared `sensitive = true` so plans show them as " +
					"`(sensitive value)`. Like every resource attribute, source_config is stored in state, so keep " +
					"state encrypted.",
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
				Required: true,
				Description: "How objects load: `MERGE`, `APPEND` or `HISTORY_TRACKING`. Not every Destination " +
					"supports every load mode, for example `HISTORY_TRACKING` on Amazon Redshift; Hevo rejects an " +
					"unsupported one at apply.",
				Validators: []validator.String{stringvalidator.OneOf("MERGE", "APPEND", "HISTORY_TRACKING")},
			},
			"schema_evolution": schema.StringAttribute{
				Required: true,
				Description: "`ALLOW_ALL` replicates new objects and fields as they appear; `ALLOW_COLUMN_LEVEL` " +
					"replicates new fields of existing objects, but not new objects; `BLOCK_ALL` replicates neither.",
				Validators: []validator.String{stringvalidator.OneOf("ALLOW_ALL", "ALLOW_COLUMN_LEVEL", "BLOCK_ALL")},
			},
			"schedule":                pipelineScheduleAttribute(),
			"failure_handling_policy": pipelineFailureHandlingPolicyAttribute(),
			"latency_alert":           pipelineLatencyAlertAttribute(),
			"source_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "The connector's variant, for example `AURORA` or `AWS_RDS_MYSQL` for a MySQL Source. The " +
					"variants and the default are in the connector's schema in `hevo_connector_metadata`. When not set, " +
					"Hevo uses the default." + replacesPipeline,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
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
				Description: "How often a `SCHEDULED` Pipeline ingests. Required with `SCHEDULED`; ignored with `ON_DEMAND`.",
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
				Optional:      true,
				Description:   "How many objects may fail under `PARTIAL`." + replacesPipeline,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplaceIfConfigured()},
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
