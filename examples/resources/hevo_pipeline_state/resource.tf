# One apply creates the Pipeline (disabled), selects its objects, then enables it.
resource "hevo_pipeline_state" "postgres" {
  pipeline_id = hevo_pipeline.postgres.id
  paused      = false

  depends_on = [hevo_pipeline_objects.postgres]
}
