# Replicate exactly these three tables; anything else selected in the Hevo UI shows as drift.
# Keys are fully qualified names (database.schema.table for Postgres): list them with the
# hevo_pipeline_objects data source.
resource "hevo_pipeline_objects" "postgres" {
  pipeline_id = hevo_pipeline.postgres.id
  select      = "listed"

  objects = {
    "shop.public.orders"   = {}
    "shop.public.payments" = { load_mode = "APPEND" }
    "shop.public.users"    = { excluded_fields = ["ssn", "date_of_birth"] }
  }
}
