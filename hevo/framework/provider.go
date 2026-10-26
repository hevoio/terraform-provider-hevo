// Package framework is the Hevo provider, built on the Terraform Plugin Framework.
package framework

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hevoio/hevo-go"
	httputils "github.com/hevoio/hevo-go/http_utils"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework/core"
	"github.com/hevoio/terraform-provider-hevo/hevo/framework/datasources"
	"github.com/hevoio/terraform-provider-hevo/hevo/framework/resources"
)

// Environment variables the provider reads when its block leaves a setting out.
const (
	EnvAccessKey = "HEVO_ACCESS_KEY"
	EnvSecretKey = "HEVO_SECRET_KEY" //nolint:gosec // the name of an environment variable, not a secret
	EnvRegion    = "HEVO_REGION"
	EnvHost      = "HEVO_HOST"
)

type hevoProvider struct {
	version string
	// mockClient replaces the HTTP client in unit tests.
	mockClient    httputils.HTTPClient
	metadataCache *sync.Map
}

type hevoProviderModel struct {
	AccessKey              types.String `tfsdk:"access_key"`
	SecretKey              types.String `tfsdk:"secret_key"`
	Region                 types.String `tfsdk:"region"`
	Host                   types.String `tfsdk:"host"`
	SkipPlanTimeValidation types.Bool   `tfsdk:"skip_plan_time_validation"`
}

var _ provider.Provider = (*hevoProvider)(nil)

// HevoProvider returns the provider factory for version.
func HevoProvider(version string) func() provider.Provider {
	return func() provider.Provider {
		return &hevoProvider{version: version, metadataCache: &sync.Map{}}
	}
}

// HevoProviderMock returns a provider whose requests go to client, for unit tests.
func HevoProviderMock(client httputils.HTTPClient) provider.Provider {
	return &hevoProvider{version: "test", mockClient: client, metadataCache: &sync.Map{}}
}

func (p *hevoProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "hevo"
	resp.Version = p.version
}

func (p *hevoProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Hevo Pipelines, Destinations and their objects through Hevo's API.",
		Attributes: map[string]schema.Attribute{
			"access_key": schema.StringAttribute{
				Optional:    true,
				Description: "The team's API access key. Defaults to the " + EnvAccessKey + " environment variable.",
			},
			"secret_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "The team's API secret key. Defaults to the " + EnvSecretKey + " environment variable.",
			},
			"region": schema.StringAttribute{
				Optional: true,
				Description: "The Hevo region of the team: one of " + strings.Join(hevo.Regions(), ", ") +
					". Defaults to the " + EnvRegion + " environment variable, then us.",
				Validators: []validator.String{
					stringvalidator.OneOf(hevo.Regions()...),
					stringvalidator.ConflictsWith(path.MatchRoot("host")),
				},
			},
			"host": schema.StringAttribute{
				Optional: true,
				Description: "The API host, used instead of region, for example http://localhost:9080. Defaults to " +
					"the " + EnvHost + " environment variable.",
			},
			"skip_plan_time_validation": schema.BoolAttribute{
				Optional: true,
				Description: "Skip checking connector configs against each connector's schema during plan, for " +
					"example to plan without network access. Mistakes then surface at apply.",
			},
		},
	}
}

func (p *hevoProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data hevoProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	settings := map[string]types.String{
		"access_key": data.AccessKey,
		"secret_key": data.SecretKey,
		"region":     data.Region,
		"host":       data.Host,
	}
	for name, value := range settings {
		if value.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Unknown provider setting",
				"The provider cannot be configured with "+name+" unknown until apply. Set it to a known value "+
					"or use the matching environment variable.")
		}
	}

	accessKey := valueOrEnv(data.AccessKey, EnvAccessKey)
	secretKey := valueOrEnv(data.SecretKey, EnvSecretKey)

	if accessKey == "" {
		resp.Diagnostics.AddAttributeError(path.Root("access_key"), "Missing access key",
			"Set access_key in the provider block or the "+EnvAccessKey+" environment variable.")
	}

	if secretKey == "" {
		resp.Diagnostics.AddAttributeError(path.Root("secret_key"), "Missing secret key",
			"Set secret_key in the provider block or the "+EnvSecretKey+" environment variable.")
	}

	if resp.Diagnostics.HasError() {
		return
	}

	client := hevo.New(accessKey, secretKey)

	if host := valueOrEnv(data.Host, EnvHost); host != "" {
		client.BaseURL(host)
	} else if region := valueOrEnv(data.Region, EnvRegion); region != "" {
		if err := client.Region(region); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("region"), "Unknown region", err.Error())

			return
		}
	}

	if p.mockClient != nil {
		client.SetHTTPClient(p.mockClient)
	}

	client.CustomUserAgent("terraform-provider-hevo/" + p.version + " terraform/" + req.TerraformVersion)

	providerData := &core.ProviderResourceData{
		Client:                 client,
		MetadataCache:          p.metadataCache,
		SkipPlanTimeValidation: data.SkipPlanTimeValidation.ValueBool(),
	}
	resp.ResourceData = providerData
	resp.DataSourceData = providerData
}

func (p *hevoProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.Destination,
		resources.Pipeline,
		resources.PipelineObjects,
		resources.PipelineState,
	}
}

func (p *hevoProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.Connectors,
		datasources.ConnectorMetadata,
		datasources.Destination,
		datasources.Destinations,
		datasources.Pipeline,
		datasources.Pipelines,
		datasources.PipelineJobs,
	}
}

// valueOrEnv returns the configured value, else the environment variable.
func valueOrEnv(value types.String, env string) string {
	if v := value.ValueString(); v != "" {
		return v
	}

	return os.Getenv(env)
}
