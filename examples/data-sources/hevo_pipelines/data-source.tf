# Every running Pipeline that loads into one Destination.
data "hevo_pipelines" "into_snowflake" {
  destination_id = hevo_destination.snowflake.id
  states         = ["Initialized"]
}
