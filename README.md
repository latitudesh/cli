
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

  

`lsh login <API_KEY>`

  

List your servers

  

`lsh servers list`

  

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

lsh block list --project <PROJECT_ID>

```

Mount block storage to a server (auto-generates/detects NQN and executes mount automatically)

```bash

# Run directly on the server with sudo - NQN will be auto-detected or generated
# and mount will be executed automatically
sudo lsh block mount --id <BLOCK_ID> --subsystem-nqn <CONNECTOR_ID>

# Example with actual values
# Simple! Just provide the block ID
sudo lsh block mount --id blk_abc123

# Or override subsystem NQN if needed
sudo lsh block mount \
  --id blk_abc123 \
  --subsystem-nqn nqn.2001-07.com.ceph:YOUR-CONNECTOR-ID

# This will automatically:
# 1. Fetch block storage details and connector_id from API
# 2. Install nvme-cli if not present (apt/yum/dnf)
# 3. Auto-detect NQN from /etc/nvme/hostnqn (or generate if missing)
# 4. Send client NQN to API to authorize access to the storage
# 5. Load required NVMe modules (nvme_tcp)
# 6. Connect to the NVMe-oF target using connector_id
# 7. Verify the connection and show available devices
# 8. Provide instructions for formatting and mounting

# Required parameters:
# --id: Block storage ID

# How it works:
# - Block ID: Used to auto-fetch connector_id (subsystem NQN)
# - Client NQN: Auto-detected and sent to API to authorize this client
# - Subsystem NQN: Auto-fetched from block storage's connector_id
# - Gateway: Defaults to 67.213.118.147:4420

```


## Troubleshooting
If you encounter any problems when installing the CLI with the installation script, you can use the command below to uninstall the CLI.

```bash

curl -sSL  https://raw.githubusercontent.com/latitudesh/lsh/main/uninstall.sh | bash

```

## Docs

  

For more information, see the documentation.

- [lsh Docs](https://www.latitude.sh/docs/cli)

- [Product Docs](https://www.latitude.sh/docs)

- [API Docs](https://docs.latitude.sh/reference)

- [SDKs & Postman Collection](https://docs.latitude.sh/reference/client-libraries)

  

## Provide feedback and contribute

  

- [Open an issue](https://github.com/latitudesh/lsh/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc) for questions, feedback, bug reports or feature requests.

- We welcome pull requests for bug fixes, new features, and improvements to the examples.
