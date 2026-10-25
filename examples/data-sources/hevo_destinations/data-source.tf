data "hevo_destinations" "all" {}

output "destination_names" {
  value = data.hevo_destinations.all.destinations[*].name
}
