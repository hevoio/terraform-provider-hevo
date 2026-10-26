# Import a Pipeline by its numeric id. Hevo does not return a Source's settings: write
# source_config (and source_auth for an OAuth app) in the configuration. The next apply records
# source_config as written without sending it, and sends source_auth.
terraform import hevo_pipeline.hubspot 42
