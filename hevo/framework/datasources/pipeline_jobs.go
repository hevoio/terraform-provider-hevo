package datasources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hevoio/hevo-go"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core/model"
	hevoschema "github.com/hevoio/terraform-provider-hevo/hevo/framework/core/schema"
)

const defaultJobsLimit = 20

// PipelineJobs is the hevo_pipeline_jobs data source.
func PipelineJobs() datasource.DataSource {
	return &lookup{typeName: "pipeline_jobs", schema: hevoschema.PipelineJobsDatasourceSchema, read: readPipelineJobs}
}

func readPipelineJobs(ctx context.Context, client *hevo.Client, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data model.PipelineJobsDatasourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	pipelineID, ok := parseID(data.PipelineID.ValueString(), "pipeline_id", &resp.Diagnostics)
	if !ok || resp.Diagnostics.HasError() {
		return
	}

	data.Jobs, data.Objects = []model.JobModel{}, []model.JobObjectModel{}

	if jobID := data.JobID.ValueString(); jobID != "" {
		readOneJob(ctx, client, pipelineID, jobID, &data, resp)
	} else {
		readJobs(ctx, client, pipelineID, &data, resp)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func readOneJob(
	ctx context.Context,
	client *hevo.Client,
	pipelineID int64,
	jobID string,
	data *model.PipelineJobsDatasourceModel,
	resp *datasource.ReadResponse,
) {
	job, err := client.GetPipelineJob().PipelineID(pipelineID).JobID(jobID).Do(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read job", err.Error())

		return
	}

	data.Jobs = append(data.Jobs, model.NewJob(job.Data))
	cursor := ""

	for {
		call := client.ListPipelineJobObjects().PipelineID(pipelineID).JobID(jobID).Limit(pageLimit)
		if cursor != "" {
			call.Cursor(cursor)
		}

		page, err := call.Do(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Unable to read the job's objects", err.Error())

			return
		}

		for _, o := range page.Data {
			data.Objects = append(data.Objects, model.NewJobObject(o))
		}

		if !page.HasMore || page.NextCursor == "" {
			return
		}

		cursor = page.NextCursor
	}
}

func readJobs(ctx context.Context, client *hevo.Client, pipelineID int64, data *model.PipelineJobsDatasourceModel, resp *datasource.ReadResponse) {
	limit := defaultJobsLimit
	if !data.Limit.IsNull() {
		limit = int(data.Limit.ValueInt64())
	}

	cursor := ""

	for len(data.Jobs) < limit {
		call := client.ListPipelineJobs().PipelineID(pipelineID).Limit(min(limit-len(data.Jobs), pageLimit))
		if cursor != "" {
			call.Cursor(cursor)
		}

		if statuses := stringValues(data.Statuses); len(statuses) > 0 {
			call.Statuses(statuses...)
		}

		if jobTypes := stringValues(data.Types); len(jobTypes) > 0 {
			call.Types(jobTypes...)
		}

		page, err := call.Do(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Unable to list jobs", err.Error())

			return
		}

		for _, j := range page.Data {
			data.Jobs = append(data.Jobs, model.NewJob(j))
		}

		if !page.HasMore || page.NextCursor == "" {
			return
		}

		cursor = page.NextCursor
	}
}

func stringValues[T interface{ ValueString() string }](values []T) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, v.ValueString())
	}

	return out
}
