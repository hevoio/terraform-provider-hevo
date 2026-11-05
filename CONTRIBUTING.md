# Contributing

## Layout

| Path | What it holds |
| --- | --- |
| `main.go` | Serves the provider. |
| `hevo/framework/provider.go` | The provider block and the list of resources and data sources. |
| `hevo/framework/core/` | The client handed to every resource, and shared helpers. `schema/` and `model/` hold each resource's schema and its Terraform model. |
| `hevo/framework/resources/`, `datasources/` | One file per resource or data source, with its mock test beside it. |
| `hevo/tests/mock/` | Runs the provider in Terraform against an in-memory API. |
| `hevo/tests/e2e/` | Acceptance tests against a real account (`make testacc`). |
| `templates/`, `examples/`, `docs/` | Documentation sources and the generated `docs/`. |

## Adding a resource

1. Add the schema in `core/schema/<name>.go` and the model in `core/model/<name>.go`.
2. Add the resource in `resources/<name>.go` and register it in `provider.go`.
3. Add a mock test in `resources/<name>_test.go` and an acceptance test in `hevo/tests/e2e/`.
4. Add `examples/resources/hevo_<name>/resource.tf` and run `make docs`.

## Rules

- Names follow Hevo's API: Pipeline, Source, Destination, object, job, connector.
- Every attribute has a description; `make docs` turns them into the registry documentation.
- Run `make lint test docs` before opening a PR. Name branches `feature/<description>` or
  `fix/<description>`, and write PR titles as `type(scope): description`, for example
  `fix(pipeline): keep the schedule when only the name changes`. PRs are squash-merged, so the
  title becomes the commit message.
