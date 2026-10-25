resource "hevo_pipeline" "hubspot" {
  name                = "HubSpot to Snowflake"
  source_name         = "HubSpot"
  source_connector_id = "hubspot"

  # Keys and value types follow the connector's schema:
  # data "hevo_connector_metadata" with kind = "source" and connector_id = "hubspot".
  source_config = {
    authentication_method    = "PRIVATE_APP_TOKEN"
    account_id               = 12345678
    private_app_access_token = var.hubspot_token
  }

  destination_id     = hevo_destination.snowflake.id
  destination_prefix = "hubspot_"
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
