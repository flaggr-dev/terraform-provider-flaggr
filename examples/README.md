# Examples

These examples are embedded in the provider documentation (`docs/`) by
[tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs). You can
also copy them into your own configuration.

tfplugindocs reads these files; other `*.tf` files are ignored:

- `provider/provider.tf`: the provider index page
- `resources/<resource name>/resource.tf`: the resource page
- `resources/<resource name>/import.sh`: the `terraform import` command on the resource page
- `resources/<resource name>/import-by-string-id.tf`: the `import` block on the resource page
- `data-sources/<data source name>/data-source.tf`: the data source page

After changing an example, run `make docs` and commit the regenerated `docs/`.
