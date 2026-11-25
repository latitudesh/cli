# Contributing

## Build and test

```bash
make build
make test
```

`go test ./...` runs on every pull request. It needs no API token: every test
runs offline against fakes.

## SDK coverage

The CLI is built on [`latitudesh-go-sdk`](https://github.com/latitudesh/latitudesh-go-sdk).
`sdk-coverage.yaml` at the repository root records, for every SDK method,
which command calls it or why it deliberately has none. A method with no entry
is **pending**, and a group with no entry at all is a new command group waiting
to be added. The file is an exception list, not a work queue: nothing waits on
someone writing a line first.

See what is pending against the pinned SDK:

```bash
make coverage-report
go run ./cmd/sdkcoverage report -format markdown
go run ./cmd/sdkcoverage report -format json | jq '.pending'
```

Check the manifest against a newer SDK without touching `go.mod`:

```bash
dir=$(go mod download -json github.com/latitudesh/latitudesh-go-sdk@latest | jq -r '.Dir')
go run ./cmd/sdkcoverage report -sdk-dir "$dir" -format text
```

The report lists one suggested command per operation, following the CLI's
own conventions rather than the SDK's method names:

- a noun the SDK flattened into several CRUD methods (`CreateLksNodePool`,
  `ListLksNodePools`, ...) becomes a subgroup with plain verbs
  (`lks node-pools create`), like `kubernetes clusters create` today; a noun
  that is only listed is a reference collection and becomes a subgroup too
  (`lks sites list`, like `kubernetes versions list`); a noun that is only
  got and updated is a setting of its parent and stays a hyphenated leaf
  (`servers get-deploy-config`);
- one operation reachable through two routes (`POST /virtual_machine_backups`
  and `POST /virtual_machines/{id}/backups`) folds into a single suggestion,
  since the CLI exposes it as one command with a flag for the parent. The
  `methods` array in the JSON output names every SDK method behind a
  suggestion.

These are hints for the tracking issue and the scaffold prompt, not
decisions: the final name is settled when the command is written and mapped
in the manifest. When the UX is decided before the code and the automatic
name would be wrong, record it under `groups.<Group>.suggested.<Method>`: a
BGP session is a server announcing an elastic IP, so it is
`elastic-ips attach --server`, not a `bgp-sessions` subgroup. A suggestion
names a command that does not exist yet. When the method is to be served by
a command the CLI already has (listing BGP sessions inside `elastic-ips
get`), record it under `groups.<Group>.extends.<Method>` instead: the path
must exist, the method stays pending, and the report marks the command
*extend* (`"extends": true` in the JSON) so the scaffold grows it rather than
creating a new one. Do not map it under `commands` until the command
actually calls the method; that is what counts it as covered.

Inspect the two inputs the tool reconciles:

```bash
go run ./cmd/sdkcoverage groups
go run ./cmd/sdkcoverage commands
```

### Keeping the manifest honest

`TestCLISDKCoverage` (in `cmd/`) runs with the rest of the suite and fails only
on contradictions:

- a command path in the manifest that the CLI does not register;
- a method or group the SDK no longer exposes;
- a leaf command that no method claims and that is not listed under
  `commands_without_sdk`;
- a method both mapped and excluded, or an exclusion without a known reason;
- a `suggested` command that already exists, or an `extends` command that
  does not, or an `extends` entry for a method that is already mapped (the
  extension landed: drop the entry);
- two SDK groups under one command group proposing the same command.

An SDK bump that adds methods is not a failure. When you add, rename or remove
a command, update `sdk-coverage.yaml` in the same PR:

- a new command that calls an SDK method goes under
  `groups.<Group>.commands.<Method>`;
- a group whose commands belong under an existing family (VM backups under
  `virtual-machines`) declares it with `groups.<Group>.command_group`, so the
  report and the scaffold place them next to their siblings. Set it too when
  the first commands you map for a group all sit in one of its subgroups
  (only `lks node-pools ...` mapped for an `lks` group): the group is
  otherwise inferred as the shortest parent of its mapped commands, and the
  remaining suggestions would land inside that subgroup;
- a command that calls no SDK method (local state, self-update) goes under
  `commands_without_sdk`;
- a method you looked at and chose not to expose goes under
  `groups.<Group>.excluded.<Method>` with a `reason` (`api_constraint`,
  `product_decision` or `deprecated`) and, ideally, a note;
- a whole product that is not ready for the CLI (a preview behind a feature
  flag) goes under `groups.<Group>.excluded_group` with the same fields.

`make coverage-check` runs the same check from the command line.
