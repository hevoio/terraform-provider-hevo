# Import by the Pipeline's id. The selection is read as select = "listed" with the objects
# currently selected, and each object's load_mode when it differs from the Pipeline's.
# excluded_fields are not read back: write them in the configuration, and the next apply sends
# them again, which changes nothing in Hevo.
terraform import hevo_pipeline_objects.postgres 42
