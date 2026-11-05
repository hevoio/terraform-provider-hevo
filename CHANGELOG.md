# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-11-05

First release. Manages Hevo Destinations, Pipelines and the objects each Pipeline replicates.

### Added

#### Provider

- `hevo` provider: authenticates with the team's API access key and secret key (`access_key`,
  `secret_key`, or `HEVO_ACCESS_KEY` and `HEVO_SECRET_KEY`). `host` (or `HEVO_HOST`) selects the
  region and defaults to `https://us.hevodata.com`.
- `skip_plan_time_validation`: skips the plan-time checks that read connector schemas and objects
  from Hevo, for example to plan without network access. Those mistakes then surface at apply.

#### Resources

- `hevo_destination`: a Destination. Hevo tests the connection on create and on every config
  change. Only changed `config` keys are sent on update. Changing `connector_id` replaces it.
- `hevo_pipeline`: a Pipeline, created together with its Source. It is always created disabled.
  Edits run asynchronously in Hevo, and the provider waits for each one to be applied.
- `hevo_pipeline_objects`: the objects a Pipeline replicates, as a desired set.
  `select = "listed"` replicates exactly the objects listed; `select = "all"` replicates every
  object except those disabled in `objects`. Per-object `load_mode` and `excluded_fields`.
- `hevo_pipeline_state`: enables (`paused = false`) or pauses a Pipeline and waits for it to
  settle. Kept apart from `hevo_pipeline` so one apply can create a Pipeline, select its objects
  and then enable it.

#### Data sources

- `hevo_connectors`: the connectors Hevo offers for Sources or for Destinations.
- `hevo_connector_metadata`: one connector's JSON Schema, with its required `config` keys, its
  write-only (credential) keys and, for a source connector, its variants (`source_types`).
- `hevo_destination`: one Destination, with its config as Hevo returns it (credentials masked).
- `hevo_destinations`: the team's Destinations.
- `hevo_pipeline`: one Pipeline's settings, state and health.
- `hevo_pipelines`: the team's Pipelines, filtered by Destination, source connector or state.
- `hevo_pipeline_objects`: a Pipeline's objects with the fully qualified names
  `hevo_pipeline_objects` takes, filtered by status.
- `hevo_pipeline_jobs`: a Pipeline's newest jobs with their event and object counts, filtered by
  status and type, or one job with what it did for each object.

#### Behavior

- Plan-time validation: `hevo_destination` `config` and `hevo_pipeline` `source_config` are checked
  against the connector's schema during plan: unknown keys, value types, allowed values, patterns,
  and required keys, including keys required only under a choice (for example a password when
  `authentication_type` is `BASIC`). An unset key counts as its schema default. `hevo_pipeline`
  also checks `schedule`, `failure_handling_policy` and `latency_alert` as Hevo does.
  `hevo_pipeline_objects` checks during plan that each object an update adds exists.
- Replacement warning: the plan warns when a change replaces a Pipeline, which loads all
  historical data again. `source_connector_id`, `source_type`, `destination_id`,
  `destination_prefix`, `replication_type` and `failure_handling_policy.object_failure_threshold`
  replace it.
- Import: all four resources import by numeric id (`hevo_pipeline_objects` and
  `hevo_pipeline_state` by the Pipeline's id). Credentials are never read back: add them to the
  configuration after an import. After a `hevo_pipeline` import, `source_config` may be left out;
  when set, the first apply records it without sending it.
- Drift: `hevo_destination` detects drift on the `config` keys you set. `hevo_pipeline_objects`
  shows objects selected or deselected outside Terraform as drift. Hevo does not return a Source's
  settings, so `source_config` and `source_auth` are sent as written and not checked for drift.
- Credentials: write-only keys keep the value you set, since Hevo masks them in every read.
  `source_auth` (for a Source that uses your own OAuth app) is sensitive and never read back.
- `force_delete` on `hevo_destination`: deletes a Destination that Pipelines still load into,
  together with those Pipelines and their Sources. Destination tables are kept. Without it, the
  destroy fails and names the Pipelines.
- Busy Pipelines: a change refused because the Pipeline is mid-transition or has an edit open is
  retried for up to 5 minutes.
- Create recovery: when a `hevo_destination` or `hevo_pipeline` create fails with a timeout, a
  network error or a 5xx, the provider looks the resource up by name. If Hevo created it anyway,
  it is recorded in state with a warning instead of being left unmanaged.
- Removing `hevo_pipeline_objects` or `hevo_pipeline_state` from the configuration leaves the
  Pipeline's selection and state as they are.
- Timeouts: `hevo_pipeline` takes `create`, `update` and `delete` timeouts;
  `hevo_pipeline_objects` and `hevo_pipeline_state` take `create` and `update`.

[Unreleased]: https://github.com/hevoio/terraform-provider-hevo/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/hevoio/terraform-provider-hevo/releases/tag/v1.0.0
