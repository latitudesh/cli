
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

curl -sSL  https://raw.githubusercontent.com/latitudesh/cli/main/install.sh | bash

```

#### Windows is not supported yet

##

### From Github

Visit the Releases page and select any version you want to download.

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
