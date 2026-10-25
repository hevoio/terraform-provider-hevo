package e2e_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework"
)

// providerFactories runs the provider in-process for acceptance tests.
var providerFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"hevo": providerserver.NewProtocol6WithError(framework.HevoProvider("acceptance")()),
}

// preCheck skips nothing: acceptance tests run only with TF_ACC set, and then need a real account.
func preCheck(t *testing.T) {
	t.Helper()

	for _, env := range []string{framework.EnvAccessKey, framework.EnvSecretKey} {
		if os.Getenv(env) == "" {
			t.Fatalf("%s must be set for acceptance tests", env)
		}
	}

	if os.Getenv(framework.EnvHost) == "" && os.Getenv(framework.EnvRegion) == "" {
		t.Fatalf("one of %s or %s must be set for acceptance tests", framework.EnvHost, framework.EnvRegion)
	}
}
