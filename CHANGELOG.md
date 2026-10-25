# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Provider block: `access_key`, `secret_key`, `region` or `host` (or the `HEVO_*` environment
  variables), and `skip_plan_time_validation`.
- `hevo_destination` resource: create, update in place (only changed `config` keys are sent),
  import by id, drift detection on the `config` keys you set, plan-time validation against the
  connector's schema, and `force_delete` to delete a Destination that Pipelines still use.
- `hevo_connectors` data source: the source or destination connectors Hevo offers.
- `hevo_connector_metadata` data source: one connector's JSON Schema, with its required and
  write-only `config` keys.
- `hevo_pipeline` resource: creates a Pipeline with its Source, always disabled; waits for each
  edit to be applied and retries while the Pipeline is busy; `source_connector_id`,
  `destination_id`, `destination_prefix` and `replication_type` replace it, with a warning;
  import by id. Hevo does not return a Source's settings, so `source_config` and `source_auth`
  are sent as written and not checked for drift.
- `hevo_pipeline`, `hevo_pipelines`, `hevo_destination` and `hevo_destinations` data sources.
- `hevo_pipeline_objects` resource: a Pipeline's objects as a desired set (`select = "listed"` or
  `"all"`), with per-object load mode and excluded fields; objects selected or deselected outside
  Terraform show as drift. Removing it leaves the selection unchanged.
- `hevo_pipeline_state` resource: enables (`paused = false`) or pauses a Pipeline and waits for
  it to settle. Removing it leaves the Pipeline as it is.
- `hevo_pipeline_jobs` data source: a Pipeline's newest jobs with their counts, filtered by
  status and type, or one job with what it did for each object.
