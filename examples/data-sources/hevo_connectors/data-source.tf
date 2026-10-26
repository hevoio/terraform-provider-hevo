data "hevo_connectors" "destinations" {
  kind = "destination"
}

output "destination_connector_ids" {
  value = data.hevo_connectors.destinations.connectors[*].connector_id
}
