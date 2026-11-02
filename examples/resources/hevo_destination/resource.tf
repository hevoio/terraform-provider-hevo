resource "hevo_destination" "snowflake" {
  name         = "Snowflake"
  connector_id = "snowflake"

  # Keys and value types follow the connector's schema:
  # data "hevo_connector_metadata" with connector_id = "snowflake".
  config = {
    account_url         = "https://acme.snowflakecomputing.com"
    warehouse           = "LOAD_WH"
    database_name       = "ANALYTICS"
    database_user       = "HEVO"
    database_password   = var.snowflake_password
    authentication_type = "BASIC"
  }
}

# Credentials come from a sensitive variable, so plans show them as (sensitive value).
variable "snowflake_password" {
  type      = string
  sensitive = true
}
