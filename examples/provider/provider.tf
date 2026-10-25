terraform {
  required_providers {
    hevo = {
      source = "hevoio/hevo"
    }
  }
}

# Prefer the HEVO_ACCESS_KEY and HEVO_SECRET_KEY environment variables to writing the keys here.
provider "hevo" {
  region = "us"
}
