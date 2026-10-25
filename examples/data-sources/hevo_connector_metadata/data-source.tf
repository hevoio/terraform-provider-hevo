data "hevo_connector_metadata" "snowflake" {
  kind         = "destination"
  connector_id = "snowflake"
}

locals {
  snowflake_config_schema = jsondecode(data.hevo_connector_metadata.snowflake.schema_json).properties.config
}

output "snowflake_required_keys" {
  value = data.hevo_connector_metadata.snowflake.required_config_keys
}
