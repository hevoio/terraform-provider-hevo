package datasources

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hevoio/hevo-go"
	"github.com/hevoio/hevo-go/pipelines"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core/model"
	hevoschema "github.com/hevoio/terraform-provider-hevo/hevo/framework/core/schema"
)

// lookup is the shared shape of the four lookup data sources: a type name, a schema and a read.
type lookup struct {
	core.ProviderDatasource

	typeName string
	schema   func() schema.Schema
	read     func(ctx context.Context, client *hevo.Client, req datasource.ReadRequest, resp *datasource.ReadResponse)
}

var _ datasource.DataSourceWithConfigure = (*lookup)(nil)

func (d *lookup) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.typeName
}

func (d *lookup) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = d.schema()
}

func (d *lookup) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.GetClient() == nil {
		core.AddUnconfiguredClientError(&resp.Diagnostics)

		return
	}

	d.read(ctx, d.GetClient(), req, resp)
}

// Destination is the hevo_destination data source.
func Destination() datasource.DataSource {
	return &lookup{typeName: "destination", schema: hevoschema.DestinationDatasourceSchema, read: readDestination}
}

// Destinations is the hevo_destinations data source.
func Destinations() datasource.DataSource {
	return &lookup{typeName: "destinations", schema: hevoschema.DestinationsDatasourceSchema, read: readDestinations}
}

// Pipeline is the hevo_pipeline data source.
func Pipeline() datasource.DataSource {
	return &lookup{typeName: "pipeline", schema: hevoschema.PipelineDatasourceSchema, read: readPipeline}
}

// Pipelines is the hevo_pipelines data source.
func Pipelines() datasource.DataSource {
	return &lookup{typeName: "pipelines", schema: hevoschema.PipelinesDatasourceSchema, read: readPipelines}
}

func parseID(value, attr string, diags *diag.Diagnostics) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		diags.AddAttributeError(path.Root(attr), "Invalid id", "Expected a numeric id, got "+strconv.Quote(value)+".")

		return 0, false
	}

	return id, true
}

func readDestination(ctx context.Context, client *hevo.Client, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data model.DestinationDatasourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	id, ok := parseID(data.ID.ValueString(), "id", &resp.Diagnostics)
	if !ok || resp.Diagnostics.HasError() {
		return
	}

	found, err := client.GetDestination().DestinationID(id).Do(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Destination", err.Error())

		return
	}

	resp.Diagnostics.Append(data.ReadFromResponse(found.Data)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func readDestinations(ctx context.Context, client *hevo.Client, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	data := model.DestinationsDatasourceModel{Destinations: []model.DestinationSummaryModel{}}
	cursor := ""

	for {
		call := client.ListDestinations().Limit(pageLimit)
		if cursor != "" {
			call.Cursor(cursor)
		}

		page, err := call.Do(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Unable to list Destinations", err.Error())

			return
		}

		for _, d := range page.Data {
			data.Destinations = append(data.Destinations, model.NewDestinationSummary(d))
		}

		if !page.HasMore || page.NextCursor == "" {
			break
		}

		cursor = page.NextCursor
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func readPipeline(ctx context.Context, client *hevo.Client, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data model.PipelineSummaryModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	id, ok := parseID(data.ID.ValueString(), "id", &resp.Diagnostics)
	if !ok || resp.Diagnostics.HasError() {
		return
	}

	found, err := client.GetPipeline().PipelineID(id).Do(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Pipeline", err.Error())

		return
	}

	data = model.NewPipelineSummary(found.Data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func readPipelines(ctx context.Context, client *hevo.Client, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data model.PipelinesDatasourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	states := stringValues(data.States)

	var destinationID int64

	if v := data.DestinationID.ValueString(); v != "" {
		id, ok := parseID(v, "destination_id", &resp.Diagnostics)
		if !ok {
			return
		}

		destinationID = id
	}

	data.Pipelines = []model.PipelineSummaryModel{}
	cursor := ""

	for {
		page, err := listPipelinesCall(client, cursor, states, data.SourceConnectorID.ValueString(), destinationID).Do(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Unable to list Pipelines", err.Error())

			return
		}

		for _, p := range page.Data {
			data.Pipelines = append(data.Pipelines, model.NewPipelineSummary(p))
		}

		if !page.HasMore || page.NextCursor == "" {
			break
		}

		cursor = page.NextCursor
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// listPipelinesCall builds one page's call with the filters that are set.
func listPipelinesCall(client *hevo.Client, cursor string, states []string, connectorID string, destinationID int64) *pipelines.ListPipelinesService {
	call := client.ListPipelines().Limit(pageLimit)
	if cursor != "" {
		call.Cursor(cursor)
	}

	if len(states) > 0 {
		call.States(states...)
	}

	if connectorID != "" {
		call.ConnectorID(connectorID)
	}

	if destinationID != 0 {
		call.DestinationID(destinationID)
	}

	return call
}
