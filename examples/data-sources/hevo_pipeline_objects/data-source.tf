# The names to use as keys in the hevo_pipeline_objects resource.
data "hevo_pipeline_objects" "postgres" {
  pipeline_id = hevo_pipeline.postgres.id
}

output "object_names" {
  value = data.hevo_pipeline_objects.postgres.objects[*].fully_qualified_name
}
