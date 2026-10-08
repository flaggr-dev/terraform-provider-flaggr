# Terraform Provider for Flaggr

[![Tests](https://github.com/flaggr-dev/terraform-provider-flaggr/actions/workflows/test.yml/badge.svg)](https://github.com/flaggr-dev/terraform-provider-flaggr/actions/workflows/test.yml)
[![Release](https://img.shields.io/github/v/release/flaggr-dev/terraform-provider-flaggr)](https://github.com/flaggr-dev/terraform-provider-flaggr/releases/latest)

Manage [Flaggr](https://flaggr.dev) with Terraform. Flaggr is an OpenFeature-compatible feature flag platform. This provider manages organizations and their members, projects, environments, services, feature flags, alert channels and rules, and metric sources.

- **Provider documentation:** [Terraform Registry](https://registry.terraform.io/providers/flaggr-dev/flaggr/latest/docs), also in [`docs/`](docs/)
- **Flaggr documentation:** [flaggr.dev/docs](https://flaggr.dev/docs), including the [infrastructure-as-code guide](https://flaggr.dev/docs/guides/infrastructure-as-code)

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/install) 1.0 or later (`import` blocks need 1.5 or later)
- A Flaggr account and an API token (see [Authentication](#authentication))

## Installation

Add the provider to `required_providers` and run `terraform init`:

```hcl
terraform {
  required_providers {
    flaggr = {
      source  = "flaggr-dev/flaggr"
      version = "~> 0.1"
    }
  }
}
```

Release binaries are built by this repository's [Release workflow](.github/workflows/release.yml) and signed with the provider's GPG key, which `terraform init` checks against the key registered in the Terraform Registry.

## Authentication

The provider sends a Flaggr API token to the Flaggr API. Set it in the `FLAGGR_API_TOKEN` environment variable (recommended) or with the `api_token` argument.

- A **personal access token** (`fgp_…`, recommended) acts as you in every project and organization you can access, limited to the tier you chose when you created it; your own roles still apply. Create one in Flaggr under [**Profile → Personal access tokens**](https://flaggr.dev/console/profile#personal-access-tokens).
- A **project API token** (`fgr_…`) works only for resources inside its own project: it can't create or delete projects, or manage organizations and their members.

| What Terraform manages | Personal access token you need |
|------------------------|--------------------------------|
| Services, flags, alert channels and rules, metric sources in existing projects | **Write** tier |
| Organizations, organization members, environments, and creating or changing projects | **Admin** tier |
| Deleting projects or organizations (`terraform destroy`, or removing a `flaggr_project` or `flaggr_organization`) | **Admin** tier with **Allow owner actions**, and you must be an owner |

Without **Allow owner actions**, deleting a project or organization fails with HTTP 403: remove the resource from state with `terraform state rm` and delete it in the Flaggr dashboard instead. Tokens expire after 7, 30, 90, 180 or 365 days.

| Argument | Environment variable | Default | Description |
|----------|----------------------|---------|-------------|
| `api_token` | `FLAGGR_API_TOKEN` | none (required) | Personal access token (`fgp_…`) or project API token (`fgr_…`) |
| `api_url` | `FLAGGR_API_URL` | `https://flaggr.dev` | Flaggr API base URL |

Secrets inside `flaggr_alert_channel` and `flaggr_metric_source` `config` read back as `"[redacted]"` to tokens below the admin tier. The provider keeps your configured values in state, so this causes no diff; see the [provider documentation](https://registry.terraform.io/providers/flaggr-dev/flaggr/latest/docs#secrets-in-config) for details.

## Quick start

```hcl
terraform {
  required_providers {
    flaggr = {
      source  = "flaggr-dev/flaggr"
      version = "~> 0.1"
    }
  }
}

# Reads FLAGGR_API_TOKEN from the environment.
provider "flaggr" {}

resource "flaggr_project" "main" {
  name        = "My Application"
  slug        = "my-app"
  description = "Production feature flags"
}

resource "flaggr_service" "api" {
  project_id  = flaggr_project.main.id
  name        = "API"
  description = "Backend API service"
}

resource "flaggr_flag" "dark_mode" {
  project_id    = flaggr_project.main.id
  service_id    = flaggr_service.api.id
  key           = "dark-mode"
  name          = "Dark Mode"
  type          = "boolean"
  enabled       = true
  default_value = jsonencode(false)
  environment   = "production"
}
```

```bash
export FLAGGR_API_TOKEN="<your personal access token>"
terraform init
terraform apply
```

Your applications then evaluate `dark-mode` with any [Flaggr SDK](https://flaggr.dev/docs) or OpenFeature provider.

## Resources and data sources

| Resource | Manages |
|----------|---------|
| [`flaggr_organization`](docs/resources/organization.md) | An organization |
| [`flaggr_organization_member`](docs/resources/organization_member.md) | A member of an organization and their role |
| [`flaggr_project`](docs/resources/project.md) | A project, which holds services, environments and flags |
| [`flaggr_environment`](docs/resources/environment.md) | A project environment, such as `staging` or `production` |
| [`flaggr_service`](docs/resources/service.md) | An application or microservice in a project |
| [`flaggr_flag`](docs/resources/flag.md) | A feature flag in one service and environment |
| [`flaggr_alert_channel`](docs/resources/alert_channel.md) | A Slack, Discord, email, PagerDuty or webhook notification channel |
| [`flaggr_alert_rule`](docs/resources/alert_rule.md) | An alert rule on error rate, latency, evaluation volume, toggle drift or rollbacks |
| [`flaggr_metric_source`](docs/resources/metric_source.md) | An external metric source, such as Prometheus or Datadog |

Data sources: [`flaggr_organization`](docs/data-sources/organization.md), [`flaggr_project`](docs/data-sources/project.md), [`flaggr_environment`](docs/data-sources/environment.md), [`flaggr_service`](docs/data-sources/service.md) and [`flaggr_flag`](docs/data-sources/flag.md).

Every resource can be imported; each resource page gives its import ID format.

## Development

You need [Go](https://go.dev/doc/install) (the version in [`go.mod`](go.mod)) and, to regenerate the docs, [Terraform](https://developer.hashicorp.com/terraform/install).

```bash
make build      # build ./terraform-provider-flaggr
make test       # unit tests
make vet        # go vet
make vuln       # govulncheck: known vulnerabilities in code the provider calls
make docs       # regenerate docs/ with tfplugindocs
```

To try a local build, point Terraform at the directory that holds the binary with `dev_overrides` in your [CLI configuration](https://developer.hashicorp.com/terraform/cli/config/config-file) (`~/.terraformrc`), and run `terraform plan` without `terraform init`. Until you remove the block, Terraform uses your build for `flaggr-dev/flaggr` in every configuration and warns that development overrides are in effect:

```hcl
provider_installation {
  dev_overrides {
    "flaggr-dev/flaggr" = "/absolute/path/to/terraform-provider-flaggr"
  }
  direct {}
}
```

### Acceptance tests

`make testacc` runs the acceptance tests against a real Flaggr account: they create and delete projects and other resources. Set `FLAGGR_API_TOKEN` to an admin-tier personal access token created with **Allow owner actions** (and `FLAGGR_API_URL` for a deployment other than `https://flaggr.dev`). The `flaggr_organization_member` test also needs `FLAGGR_TEST_MEMBER_USER_ID`, the ID of a user who shares an organization or project with the token's user; without it, that test is skipped.

### Documentation

[tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs) generates `docs/` from the provider schema, [`examples/`](examples/) and [`templates/`](templates/). Don't edit `docs/` by hand: change the schema descriptions, examples or templates, then run `make docs` and commit the result. CI fails when `docs/` is out of date.

### Releasing

Add the release to [CHANGELOG.md](CHANGELOG.md), then push a `v*` tag (for example `v0.2.0`) from `main`. The [Release workflow](.github/workflows/release.yml) builds the binaries with GoReleaser, signs the checksums with the provider's GPG key and publishes the GitHub release, which the Terraform Registry picks up.

## Support

- Bugs and feature requests: [GitHub issues](https://github.com/flaggr-dev/terraform-provider-flaggr/issues)
- Questions about Flaggr: [hello@flaggr.dev](mailto:hello@flaggr.dev)
- Security issues: see [SECURITY.md](SECURITY.md)

## License

[MIT](LICENSE)
