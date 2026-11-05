resource "hevo_pipeline" "postgres" {
  name                = "Postgres to Snowflake"
  source_name         = "Postgres"
  source_connector_id = "SOURCE_CONNECTOR_POSTGRES"

  # Keys and value types follow the connector's schema:
  # data "hevo_connector_metadata" with kind = "source" and connector_id = "SOURCE_CONNECTOR_POSTGRES".
  source_config = {
    database_host     = "db.acme.internal"
    database_port     = 5432
    database_name     = "shop"
    database_user     = "hevo"
    database_password = var.postgres_password
    publication_key   = "hevo_publication"
    replication_slot  = "hevo_slot"
  }

  destination_id     = hevo_destination.snowflake.id
  destination_prefix = "shop_"
  replication_type   = "HISTORICAL_AND_INCREMENTAL"
  load_mode          = "MERGE"
  schema_evolution   = "ALLOW_ALL"

  schedule = {
    sync_type         = "SCHEDULED"
    frequency_minutes = 60
  }

  failure_handling_policy = {
    object_failure_level     = "PARTIAL"
    object_failure_threshold = 5
  }
}

# Credentials come from a sensitive variable, so plans show them as (sensitive value).
variable "postgres_password" {
  type      = string
  sensitive = true
}
