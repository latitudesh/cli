
# lsh

lsh is the [Latitude.sh](http://latitude.sh/) command-line interface ([CLI](https://www.latitude.sh/docs/cli)) that will help you make it easier to retrieve any data from your team or perform any action you need.

## Installation

#### MacOS / Linux / WSL

Installing the latest version

#### Homebrew

```
brew install latitudesh/tools/lsh
```

#### Installation Script

```bash
curl -fsSL https://cli.latitude.sh/install.sh | sh
```

#### Windows is not supported yet

##

### From Github

Visit the [releases page](https://github.com/latitudesh/cli/releases) and select any version you want to download.

## [](https://docs.latitude.sh/docs/getting-started)Getting Started

Log in into Latitude.sh. An API Key is required.

```bash
lsh login <API_KEY>
```

The CLI automatically detects when you use `sudo` and loads your credentials from your user directory.

List your servers

```bash
lsh servers list
```

## Commands

The list of the available commands is available [here](https://www.latitude.sh/docs/cli/commands).

## Examples

See more examples [here](https://www.latitude.sh/docs/cli/examples).

List a server with a specific hostname:

```bash

lsh servers list --hostname <HOSTNAME>

```

Create a server with Ubuntu 24:

```bash

lsh servers create --operating_system ubuntu_24_04_x64_lts --project <PROJECT_ID_OR_SLUG> --site <LOCATION> --hostname <HOSTNAME> --plan <PLAN>

```
  
List all GPU plans:

```bash

lsh plans list --gpu true

```

Check plan availability per location (one row per plan × location, with `stock_level`):

```bash
lsh plans stock
```

Show only locations that currently have stock:

```bash
lsh plans stock --in_stock
```

Filter by region, GPU, or hardware spec and export as CSV:

```bash
lsh plans stock --region "United States" --in_stock -o csv > us_plans.csv
```

Combine filters for scripting with `jq` (use `-o json` when piping — the default table is meant for humans):

```bash
lsh plans stock --gpu --ram_gte 64 -o json | jq '.[] | {plan: .plan_slug, loc: .location, stock: .stock_level}'
```

List volumes:

```bash

lsh volume list --project <PROJECT_ID>

```

Mount volume to a server (requires sudo, auto-installs nvme-cli and connects):

```bash
# First, login as normal user
lsh login <API_KEY>

# Then mount with sudo (automatically uses your credentials)
sudo lsh volume mount --id vol_abc123
```

**Why sudo is required:**

- Installs `nvme-cli` package if not present
- Loads NVMe kernel modules (`nvme_tcp`)
- Writes to `/etc/nvme/hostnqn`
- Runs privileged `nvme connect` commands

**Important:**

- Login as a **normal user** (without sudo): `lsh login <API_KEY>`
- The CLI automatically finds your credentials when you run commands with sudo
- Volume mount needs sudo for nvme-cli installation and NVMe operations

## Output formats & automation

Every `list` command can render its results in different formats, so the output
is easy to consume from scripts, CI pipelines and AI agents. See `lsh help
output-formats` for the full guide.

```bash
lsh servers list -o table            # human-readable table (default)
lsh servers list -o json             # JSON
lsh servers list -o yaml             # YAML
lsh servers list -o csv              # CSV (header + one row per item)
lsh servers list --json              # shortcut for -o json
```

Filter the structured output with a [JMESPath](https://jmespath.org/) expression
via `--query` (works with json/yaml/csv):

```bash
lsh servers list --query "[?status=='on'].id" -o json
```

Control pagination on large listings:

```bash
lsh servers list --page-size 50      # items per API page
lsh servers list --max-items 100     # stop after N items (0 = no limit)
lsh servers list --no-paginate       # first page only; next page printed to stderr
```

### Environment variables

| Variable | Purpose |
| --- | --- |
| `LSH_OUTPUT` | Default output format (`table`/`json`/`yaml`/`csv`). Precedence: `--output` flag > `LSH_OUTPUT` > config file > default. |
| `LSH_CLASSIC_OUTPUT` | Set to `true` to force the legacy plain-ASCII table. An explicit `-o json/yaml/csv` still wins over it. |
| `LATITUDESH_TOKEN` | API token; bypasses any stored profile (see `lsh help authentication`). |
| `LSH_PROFILE` | Use the named profile for the command. |
| `LSH_PROJECT` | Pre-fill `--project` so list commands don't prompt. |

## Troubleshooting

### Uninstalling

If you encounter any problems when installing the CLI with the installation script, you can use the command below to uninstall the CLI.

```bash
curl -sSL  https://raw.githubusercontent.com/latitudesh/cli/main/uninstall.sh | bash
```

### Sudo Authentication Issues

If `sudo lsh volume mount` says "API key not found":

```bash
# Make sure you've logged in as your normal user (not with sudo)
lsh login <API_KEY>

# Then try mount again
sudo lsh volume mount --id <VOLUME_ID>
```

The CLI automatically detects your username via the `SUDO_USER` environment variable and loads your config.

## Docs

For more information, see the documentation.

- [lsh Docs](https://www.latitude.sh/docs/cli)

- [Product Docs](https://www.latitude.sh/docs)

- [API Docs](https://www.latitude.sh/docs/api-reference/summary)

- [SDKs & Postman Collection](https://www.latitude.sh/docs/development/postman)

## Provide feedback and contribute

- [Open an issue](https://github.com/latitudesh/cli/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc) for questions, feedback, bug reports or feature requests.

- We welcome pull requests for bug fixes, new features, and improvements to the examples.
