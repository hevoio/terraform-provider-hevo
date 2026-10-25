# Terraform Provider for Hevo

[![Testing](https://img.shields.io/github/actions/workflow/status/hevoio/terraform-provider-hevo/testing.yml?branch=master)](https://github.com/hevoio/terraform-provider-hevo/actions)
[![License](https://img.shields.io/github/license/hevoio/terraform-provider-hevo)](./LICENSE)

Manage [Hevo](https://hevodata.com) Pipelines, Destinations and the objects each Pipeline
replicates as code, with Terraform or OpenTofu. Built on the
[Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework) and the
[hevo-go](https://github.com/hevoio/hevo-go) SDK.

## Usage

```hcl
terraform {
  required_providers {
    hevo = {
      source = "hevoio/hevo"
    }
  }
}

provider "hevo" {
  region = "us"
}
```

Set `HEVO_ACCESS_KEY` and `HEVO_SECRET_KEY` to your team's API keys. The provider's documentation
is in [docs/](./docs).

## Developing

Requirements: Go 1.26+, and Terraform 1.5+ or OpenTofu 1.6+.

```sh
make install   # builds the provider into $(go env GOPATH)/bin
make test      # unit tests, including resources run in Terraform against a mock API
make testacc   # acceptance tests against a real account (see Makefile)
make docs      # regenerates docs/ from the schema, templates/ and examples/
make lint
```

To use your build, point Terraform at it in `~/.terraformrc` (OpenTofu reads the same file):

```hcl
provider_installation {
  dev_overrides {
    "hevoio/hevo" = "/Users/<you>/go/bin"
  }
  direct {}
}
```

With `dev_overrides`, run `terraform plan` directly; `terraform init` is not needed.

## License

[MIT](./LICENSE)
