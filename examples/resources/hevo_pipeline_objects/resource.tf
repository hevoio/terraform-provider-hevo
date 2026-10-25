# Replicate exactly these three tables; anything else selected in the Hevo UI shows as drift.
resource "hevo_pipeline_objects" "postgres" {
  pipeline_id = hevo_pipeline.postgres.id
  select      = "listed"

  objects = {
    "public.orders"   = {}
    "public.payments" = { load_mode = "APPEND" }
    "public.users"    = { excluded_fields = ["ssn", "date_of_birth"] }
  }
}
