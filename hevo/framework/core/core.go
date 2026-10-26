// Package core holds what every resource and data source of the provider shares: the configured
// client and the helpers that read and write Terraform values.
package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hevoio/hevo-go"
)

type clientContainer struct {
	client                 *hevo.Client
	metadataCache          *sync.Map
	skipPlanTimeValidation bool
}

// ProviderResource is embedded in every resource to receive the configured client.
type ProviderResource struct {
	clientContainer
}

// ProviderDatasource is embedded in every data source to receive the configured client.
type ProviderDatasource struct {
	clientContainer
}

// GetClient returns the configured client; nil before Configure runs.
func (d *clientContainer) GetClient() *hevo.Client {
	return d.client
}

// GetMetadataCache returns the per-run cache of connector schemas.
func (d *clientContainer) GetMetadataCache() *sync.Map {
	return d.metadataCache
}

// GetSkipPlanTimeValidation reports whether plan-time validation against connector schemas is
// turned off.
func (d *clientContainer) GetSkipPlanTimeValidation() bool {
	return d.skipPlanTimeValidation
}

// Configure receives the provider's data.
func (d *ProviderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	d.setProviderData(&resp.Diagnostics, req.ProviderData)
}

// Configure receives the provider's data.
func (d *ProviderDatasource) Configure(
	_ context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	d.setProviderData(&resp.Diagnostics, req.ProviderData)
}

func (d *clientContainer) setProviderData(diags *diag.Diagnostics, data any) {
	// ProviderData is nil while Terraform validates configuration before the provider is
	// configured.
	if data == nil {
		return
	}

	providerData, ok := data.(*ProviderResourceData)
	if !ok {
		diags.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *core.ProviderResourceData, got %T. Please report this issue to the provider developers.", data),
		)

		return
	}

	d.client = providerData.Client
	d.metadataCache = providerData.MetadataCache
	d.skipPlanTimeValidation = providerData.SkipPlanTimeValidation
}

// AddUnconfiguredClientError reports a call made before the provider was configured.
func AddUnconfiguredClientError(diags *diag.Diagnostics) {
	diags.AddError(
		"Unconfigured Hevo client",
		"The provider was not configured before this call. Please report this issue to the provider developers.",
	)
}
