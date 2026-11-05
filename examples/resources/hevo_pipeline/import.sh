# Import a Pipeline by its numeric id. Hevo does not return a Source's settings, so source_config
# may be left out, and the Source keeps its settings. When source_config is written, the next
# apply records it as written without sending it. source_auth, for an OAuth app, is sent.
terraform import hevo_pipeline.postgres 42
