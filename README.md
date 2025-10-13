
# lsh

  

lsh is the [Latitude.sh](http://latitude.sh/) command-line interface that will help you make it easier to retrieve any data from your team or perform any action you need.

  

## [](https://dash.readme.com/project/control/v2023-06-01/docs/overview)Installation

  

#### MacOS / Linux / WSL

Installing the latest version

#### Homebrew:

```
brew install latitudesh/tools/lsh
```

#### Installation Script:

```bash

curl -sSL  https://raw.githubusercontent.com/latitudesh/lsh/main/install.sh | bash

```


#### Windows is not supported yet.

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

  

## [](https://docs.latitude.sh/docs/commands) Commands

  

The list of the available commands is available [here](https://www.latitude.sh/docs/cli/commands).

  
  

## [](https://docs.latitude.sh/docs/examples-1) Examples

  

List a server with a specific hostname

```bash

lsh servers list --hostname <HOSTNAME>

```

Create a server with Ubuntu 22 

```bash

lsh servers create --operating_system ubuntu_22_04_x64_lts --project <PROJECT_ID_OR_SLUG> --site <LOCATION> --hostname <HOSTNAME> --plan <PLAN>

```
  
List all GPU plans

```bash

lsh plans list --gpu true

```

```bash

lsh volume list --project <PROJECT_ID>

```

Mount volume to a server (requires sudo, auto-installs nvme-cli and connects)

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
curl -sSL  https://raw.githubusercontent.com/latitudesh/lsh/main/uninstall.sh | bash
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

- [API Docs](https://docs.latitude.sh/reference)

- [SDKs & Postman Collection](https://docs.latitude.sh/reference/client-libraries)

  

## Provide feedback and contribute

  

- [Open an issue](https://github.com/latitudesh/lsh/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc) for questions, feedback, bug reports or feature requests.

- We welcome pull requests for bug fixes, new features, and improvements to the examples.
