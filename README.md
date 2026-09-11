
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

`lsh volumes` and `lsh filesystems` are aliases of `lsh volume` and `lsh storage-filesystems`.

## Object storage

`lsh s3` manages buckets, objects, access keys and lifecycle rules, addressing
buckets and objects as `s3://<bucket>/<key>`. Bucket administration goes through the
Latitude API; object operations talk to the bucket's S3 endpoint directly, so
endpoint and signing region never have to be configured by hand.

`<bucket>` accepts the display name, the `bkt_` ID or the backend bucket name. If
the same display name exists in more than one place, the command lists the
candidates and you narrow it with `--project`, `-c`/`--storage-class`, `--site`
(e.g. `lsh s3 get s3://backups -c high_performance --site TYO4`), or the `bkt_` ID.

Create a bucket, upload, list and delete (in a terminal, `mb` offers to create
an S3 access key and saves it to your profile):

```bash
lsh s3 create-bucket s3://backups --region DAL --project <PROJECT_ID_OR_SLUG>
lsh s3 copy ./dump.sql s3://backups/2026/09/
lsh s3 list s3://backups/2026/09/ --human-readable --summarize
lsh s3 copy s3://backups/2026/09/dump.sql ./restore/
lsh s3 delete s3://backups/2026/09/dump.sql
```

Give an application or CI job its own scoped access key (the secret is shown
once; `-o text --query` extracts it for a secret store):

```bash
lsh s3 access-keys create --bucket backups=rw --bucket logs=readonly --name ci-deploy
lsh s3 access-keys create --bucket backups=rw --name ci-deploy -o text --query "[0].secret_access_key" | gh secret set LSH_S3_SECRET_ACCESS_KEY
lsh s3 access-keys list
lsh s3 access-keys rotate ci-deploy --delete-old
```

Expire objects automatically with lifecycle rules:

```bash
lsh s3 lifecycle create s3://logs --prefix tmp/ --expiration-days 7
lsh s3 lifecycle list s3://logs
lsh s3 lifecycle delete s3://logs expire-7d-tmp
```

Use the same buckets from rclone, mc, s3cmd or any other S3 client:

```bash
lsh s3 configure export s3://backups --format env      # also: aws | rclone | mc | s3cmd | process
```

Clean up safely (`--dry-run` only reads; multi-object deletes ask for
confirmation in a terminal and require `--yes` in CI):

```bash
lsh s3 delete s3://logs/tmp/ --recursive --dry-run
lsh s3 delete s3://logs/tmp/ --recursive --yes
lsh s3 delete-bucket s3://logs --force --yes
```

In CI, object commands authenticate with an S3 access key from the environment
instead of a saved profile:

| Variable | Purpose |
| --- | --- |
| `LSH_S3_ACCESS_KEY_ID` / `LSH_S3_SECRET_ACCESS_KEY` | S3 access key used by `cp`, `ls`, `rm`, `stat` and `presign` (both required). |
| `LSH_S3_ENDPOINT_URL` | Talk to this S3 endpoint without the API; buckets are then addressed by their backend `bucket_name`. |
| `LSH_S3_SIGNING_REGION` | SigV4 signing region when it cannot be derived from the endpoint. |
| `LSH_S3_USE_AWS_ENV` | Set to `1` to reuse `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`. |

`lsh help exit-codes` documents the exit codes (0-7, 130) that `lsh s3` returns
for scripts.

## Output formats & automation

Every `list` command can render its results in different formats, so the output
is easy to consume from scripts, CI pipelines and AI agents. See `lsh help
output-formats` for the full guide.

```bash
lsh servers list -o table            # human-readable table (default)
lsh servers list -o json             # JSON
lsh servers list -o yaml             # YAML
lsh servers list -o csv              # CSV (header + one row per item)
lsh servers list -o text             # raw values, tab-separated
lsh servers list --json              # shortcut for -o json
```

Filter the structured output with a [JMESPath](https://jmespath.org/) expression
via `--query` (works with json/yaml/csv/text):

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
| `LSH_OUTPUT` | Default output format (`table`/`json`/`yaml`/`csv`/`text`). Precedence: `--output` flag > `LSH_OUTPUT` > config file > default. |
| `LSH_CLASSIC_OUTPUT` | Set to `true` to force the legacy plain-ASCII table. An explicit `-o json/yaml/csv/text` still wins over it. |
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
