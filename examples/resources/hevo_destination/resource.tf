resource "hevo_destination" "snowflake" {
  name         = "Snowflake"
  connector_id = "snowflake"

  # Keys and value types follow the connector's schema:
  # data "hevo_connector_metadata" with connector_id = "snowflake".
  config = {
    account_url = "acme.snowflakecomputing.com"
    warehouse   = "LOAD_WH"
    database    = "ANALYTICS"
    username    = "HEVO"
    password    = var.snowflake_password
  }
}
