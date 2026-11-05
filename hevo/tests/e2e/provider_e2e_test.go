package e2e_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestProviderConfiguresE2E checks the provider configures against a real account.
func TestProviderConfiguresE2E(t *testing.T) {
	t.Parallel()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { preCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config:             `provider "hevo" {}`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: false,
			},
		},
	})
}
