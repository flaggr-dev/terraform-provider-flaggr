# Changelog

## Unreleased

BUG FIXES:

* resource/flaggr_flag, resource/flaggr_service, resource/flaggr_organization, resource/flaggr_alert_rule, resource/flaggr_environment: Removing `description` from the configuration now clears it in Flaggr. The provider left the description out of the update, so Flaggr kept the old text, the next refresh read it back, and every plan showed the removal again. An apply now sends an empty description. That also clears a description set outside Terraform (in the dashboard, for example) when the configuration has no `description`: to keep it, set `description` in the configuration or add `lifecycle { ignore_changes = [description] }`.

NOTES:

* docs/resources/project: The page now explains what an update to a project sends, that removing `description` from the configuration clears it in Flaggr (including one set in the dashboard), and that a project can't move to another organization, with the steps to re-create it in the other one.

## 0.1.1 (October 9, 2026)

BUG FIXES:

* resource/flaggr_flag: Updating a flag in place (for example `enabled`, `default_value`, `name`, `description` or `is_public`) failed with HTTP 400 `Missing required query parameter: serviceId`. The provider now names the flag's service and environment in the query string, where Flaggr looks for them.
* resource/flaggr_flag: In an environment that requires approval, Flaggr answers an update with a change request instead of changing the flag. The update now fails with **Flag change needs approval**, naming the request, and Terraform keeps the flag's previous state until the request is applied.

SECURITY:

* Release binaries are built with google.golang.org/grpc v1.84.0, golang.org/x/net v0.59.0 and golang.org/x/text v0.42.0, which fix GO-2026-4762, GO-2026-5026, GO-2026-5970, GO-2026-6061, GO-2026-6348 and GO-2026-6443. Also updated: terraform-plugin-framework v1.19.0, terraform-plugin-go v0.31.0 and terraform-plugin-testing v1.16.0. Building the provider now needs Go 1.26 or later.

NOTES:

* The `make install` target is gone. It installed an unsigned local build as version 0.1.0 under `~/.terraform.d/plugins`, which stops Terraform from downloading the released provider and fails `terraform init` in configurations whose lock file records the release. Use `make build` with `dev_overrides` instead (see README.md).

## 0.1.0 (October 8, 2026)

First public release, as `flaggr-dev/flaggr`.

FEATURES:

* **New Resource:** `flaggr_organization`
* **New Resource:** `flaggr_organization_member`
* **New Resource:** `flaggr_project`
* **New Resource:** `flaggr_environment`
* **New Resource:** `flaggr_service`
* **New Resource:** `flaggr_flag`
* **New Resource:** `flaggr_alert_channel`
* **New Resource:** `flaggr_alert_rule`
* **New Resource:** `flaggr_metric_source`
* **New Data Source:** `flaggr_organization`
* **New Data Source:** `flaggr_project`
* **New Data Source:** `flaggr_environment`
* **New Data Source:** `flaggr_service`
* **New Data Source:** `flaggr_flag`
