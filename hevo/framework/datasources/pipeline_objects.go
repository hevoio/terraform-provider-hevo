package datasources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hevoio/hevo-go"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core/model"
	hevoschema "github.com/hevoio/terraform-provider-hevo/hevo/framework/core/schema"
)

// PipelineObjects is the hevo_pipeline_objects data source.
func PipelineObjects() datasource.DataSource {
	return &lookup{typeName: "pipeline_objects", schema: hevoschema.PipelineObjectsDatasourceSchema, read: readPipelineObjects}
}

func readPipelineObjects(ctx context.Context, client *hevo.Client, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data model.PipelineObjectsDatasourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	pipelineID, ok := parseID(data.PipelineID.ValueString(), "pipeline_id", &resp.Diagnostics)
	if !ok || resp.Diagnostics.HasError() {
		return
	}

	statuses := stringValues(data.Statuses)
	if data.Statuses == nil {
		statuses = model.SelectableObjectStatuses
	}

	data.Objects = []model.PipelineObjectEntry{}
	cursor := ""

	for {
		call := client.ListPipelineObjects().PipelineID(pipelineID).Limit(pageLimit).Statuses(statuses...)
		if cursor != "" {
			call.Cursor(cursor)
		}

		page, err := call.Do(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Unable to list the Pipeline's objects", core.ErrorDetail(err))

			return
		}

		for _, o := range page.Data {
			data.Objects = append(data.Objects, model.NewPipelineObjectEntry(o))
		}

		if !page.HasMore || page.NextCursor == "" {
			break
		}

		cursor = page.NextCursor
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
