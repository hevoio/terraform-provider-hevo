// Command terraform-provider-hevo is the Terraform and OpenTofu provider for Hevo.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/hevoio/terraform-provider-hevo/hevo/framework"
)

//go:generate go tool tfplugindocs generate --provider-name hevo

// version is set by GoReleaser at build time.
var version = "dev"

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/hevoio/hevo",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), framework.HevoProvider(version), opts); err != nil {
		log.Fatal(err)
	}
}
