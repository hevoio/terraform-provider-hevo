# Import a Pipeline by its numeric id. Hevo does not return a Source's settings: write
# source_config (and source_auth for an OAuth app) in the configuration; the next apply sends
# them once.
terraform import hevo_pipeline.hubspot 42
