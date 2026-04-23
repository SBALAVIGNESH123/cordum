# Quickstart

End-to-end path from a clean checkout to a green `cordumctl demo run
quickstart`. If any step below fails on a fresh machine, file a bug —
this doc is the smoke test for the "trivial self-hosted install in
10 minutes" promise.

## Prerequisites

- **Docker** — recent enough to understand `--profile` on `docker
  compose`. The project tests against Docker 24.x and newer.
- **Go 1.24+** — required to build `cordumctl` locally. The gateway and
  supporting services come up from published images via docker compose,
  so Go is only needed for the client.
- **curl** and a shell with `jq` (optional but recommended for the
  troubleshooting section below).

Pick a working directory with ~2 GB of free disk for the image cache +
demo volumes.

## 1. Bring up the stack

```bash
./tools/scripts/quickstart.sh
```

`quickstart.sh` is the canonical bootstrap. It generates an API key
once, writes it to `.env`, and brings up the compose stack with the
`demo` profile so the greeter worker service is included. If you
prefer to run compose directly, the equivalent is:

```bash
docker compose --profile demo up -d
```

Either entry point is fine — they share the same `.env` file. Without
`--profile demo` the `greeter` worker that handles
`job.demo-quickstart.*` topics will NOT be started and the demo's
ALLOW path will time out.

## 2. Wait for the gateway to report healthy

```bash
curl -ksf https://127.0.0.1:8081/api/v1/status
```

The gateway returns 200 within ~10–30 s of first boot on a warm image
cache. Until it does, `cordumctl pack install` will fail with a
connection-refused error. Poll this endpoint in a 2s-interval, 60s-max
loop if you script the flow.

## 3. Install the demo-quickstart pack

```bash
./cordumctl pack install ./demo/quickstart/pack
```

Expected success output (paths and digest will vary):

```text
Pack installed: demo-quickstart 0.1.0
  Topics registered:
    - job.demo-quickstart.greet        (capability=demo-quickstart.greet)
    - job.demo-quickstart.delete-all   (capability=demo-quickstart.delete-all)
    - job.demo-quickstart.admin        (capability=demo-quickstart.admin)
  Workflow registered: demo-quickstart.hello
  Policy fragment applied: demo-quickstart/default
  Signing: <verified|unsigned>
```

The three topic names all start with `job.demo-quickstart.*` — the
pack-namespace invariant enforced by the gateway requires every topic
in a pack to begin with `job.<metadata.id>.` where `<metadata.id>` is
the `id` field of `demo/quickstart/pack/pack.yaml`. If you ever see
a pack refused at install with something like:

```text
topic "job.other-pack.greet" must be namespaced under job.demo-quickstart.* (derived from pack metadata.id="demo-quickstart")
```

that is the validator telling you the pack's topics and `metadata.id`
disagree. The fix is always on the pack author's side — rename the
topics or the id to match, never relax the validator.

## 4. Run the demo

```bash
./cordumctl demo run quickstart --timeout 30
```

The demo submits three jobs and prints a verdict table. A healthy run
produces all three verdicts:

| Step            | Topic                              | Verdict             |
|-----------------|------------------------------------|---------------------|
| greet           | `job.demo-quickstart.greet`        | `ALLOW`             |
| attempt_delete  | `job.demo-quickstart.delete-all`   | `DENY`              |
| escalate_admin  | `job.demo-quickstart.admin`        | `REQUIRE_APPROVAL`  |

`ALLOW` means the greeter worker produced a response. `DENY` means
the safety-kernel policy bundle refused the request before dispatch
(delete-all carries the `destructive` risk tag, which the default
policy blocks). `REQUIRE_APPROVAL` means the request is parked on the
approval queue — the demo exits 0 once all three verdicts are
observed, so approvals do not need to be resolved for the run to
succeed.

Adding `--output json > /tmp/demo.json` gives you a parseable payload
that CI can assert with `jq`:

```bash
jq -e 'any(.rows[]; .verdict=="ALLOW")
   and any(.rows[]; .verdict=="DENY")
   and any(.rows[]; .verdict=="REQUIRE_APPROVAL")' /tmp/demo.json
```

## 5. Troubleshooting

### Pack install rejected with a topic-namespace error

```text
topic "<topic>" must be namespaced under job.<id>.* (derived from pack metadata.id="<id>")
```

The three quoted tokens are: (1) the offending topic name, (2) the
expected prefix the validator derived from the manifest, and (3) the
manifest field that produced that prefix. Match them up; the
mismatch is on the pack side.

If validation reaches the gateway install path, the same message is
also emitted as a structured log line there
(`pack_install_topic_namespace_violation` with fields `pack_id`,
`topic`, `expected_prefix`, `error`). The default `cordumctl pack install`
flow runs the same validator locally first, so a bad manifest usually
fails in the CLI before the gateway ever sees it.

### `cordumctl demo run quickstart` times out on the ALLOW row

Almost always means the `greeter` worker is not running. Confirm the
compose stack came up with the `demo` profile:

```bash
docker compose ps | grep greeter
```

If the greeter container is absent, re-run `quickstart.sh` (or
`docker compose --profile demo up -d`).

### Repeat installs fail with a pack-version conflict

The pack is already installed. Use `./cordumctl pack uninstall
demo-quickstart` or re-install with `--upgrade` if the in-tree pack
has moved forward.

## 6. Teardown

```bash
docker compose --profile demo down -v
```

The `-v` flag drops the named volumes so the next `docker compose up`
starts from a clean state. Omit it to keep the gateway data (topic
registry, installed packs, audit log) between sessions.
