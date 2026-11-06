# Octopus · compact fleet TUI prototype

A standalone Go / Bubble Tea prototype of the current `nixos-octopus-builder`
workflow. **Every operation uses timers and changes in-memory mock state only.**
No subprocesses, network requests, host connections, or runtime reads of the real
builder repository occur.

## Run

```sh
./ob                              # Included Linux binary
nix develop path:. -c go run .     # Rebuild and run with the pinned Go toolchain
```

Minimum terminal: **76 × 22**. Recommended: **120 × 32**. The fleet uses one row
per machine, the available terminal height, and scrolling rather than blank rows
or a fixed-size panel. Wider terminals also show product and app-version columns.

```sh
./ob --preview                    # Print the initial fleet screen
nix develop path:. -c go build -o ob .
```

## What's represented

The embedded `hosts.json` / `lines.json` fixtures capture the 14 host pin sets and
release lines present in the builder on 2026-11-05: base, emsa, trails, demo,
app/database pairs, multi-instance hosts, and held darcee/yeou/stark repositories.
Deployment revisions, observations, targets, baseline-behind examples, and history
are **fictional**. No snapshot host is contacted. `x` expands the fixture to 120
independent mock hosts to explore the interface at fleet scale.

- **Fleet:** app differences against blessed pins, system baseline state, server
  reachability/drift, and pending repository configuration. A configuration
  revision is separate from the darcee commit.
- **Lines:** master and v42, stable → v42, branch heads, VM checks, and unit reports.
- **System:** the independent nixos-26.05 baseline, coverage, and baseline preparation.
- **VM:** mock allinone start, persistent disk, logs, maintenance (503 HTML/JSON), reset.
- **Activity:** action results and cancellation/failure records, with scrolling.

**Update apps** adopts blessed versions and clears holds, preserving system pins.
`H` in its review selects untested branch heads. **Patch system** adopts the system
baseline, preserving every application pin. Both create pending configuration
changes; **Deploy** activates the complete configuration and records a mock tag.
`B` in deployment review selects build-only, which changes no host state.

**Hold repo** targets one repository of one instance; choose the instance in host
details with `i`, then press `o`. Use `[` / `]` to select a repository, Backspace to
edit the revision, and enter a hex commit or branch offset such as `~1`.

Blessing a line or preparing a baseline simulates seeded VM service/database
checks. These checks block completion on simulated failure. Application unit
failures are reported without blocking, matching the current builder. `T` in
review skips tests and marks the result untested. Baseline preparation never
patches hosts automatically; blessing never updates hosts automatically.

## Fleet controls

| Key | Action |
| --- | --- |
| Tab / Shift+Tab, 1–5 | Switch views |
| ↑/↓, j/k | Move selection |
| PgUp/PgDn, Home/End, g/G | Page or jump through lists |
| / | Search host, product, instance, line, app/system/server/repo state |
| Enter / Esc while searching | Apply / clear filter |
| Space | Select or deselect current host |
| A / c | Select all filtered hosts / clear all selections |
| Enter | Inspect host and its instance repository pins |
| i | Focus next instance in host details |
| u / p / d | Review app update / system patch / deployment |
| o | Review repository hold for the focused host and instance |
| b | Review blessing selected line / preparing baseline |
| s / m / l / R in VM | Start / maintenance / logs / review disk reset |
| x | Expand to 120 mock hosts |
| f | Toggle simulated operation failure at step 3 |
| r | Reset all mock state |
| Esc | Back / cancel remaining queued work / clear fleet filter |
| q / Ctrl+C | Quit (q is text when editing search or a pin) |

For bulk actions, select hosts with Space or filter and press A. Review lists the
scope before starting. **Only selected hosts matching the current filter** are
included; if none are selected in the filter, the focused host is used. The queue
runs sequentially and stops on failure. Completed hosts retain their changes;
the current and remaining hosts stay unchanged on failure/cancellation.
After patching a `behind` filter, those hosts may disappear from that filter:
Esc clears it while retaining selection, so `d` can deploy the same hosts.
Repository holds always target just the focused host, regardless of selection.

Try: `x` → `/emsa` → Enter → `A` → `p` → Enter. Return, clear the filter with Esc,
then `d` → Enter to simulate deploying that selected group. Inspect `multi` to try
per-instance holds. Press `3` then `b` to prepare a new independent baseline.

All state resets when the process exits. This explores UX and orchestration
semantics; it does not reproduce real deployment failure recovery or execute CVE
scans. Dependencies bundled by an application's pinned toolchain remain the
application update process's responsibility.

## Development

```sh
nix develop path:. -c go test ./...
nix develop path:. -c go vet ./...
```

`main.go` starts the app, `model.go` owns mock state and timer-driven transitions,
`view.go` renders compact screens, and the JSON fixtures supply repository-derived
pin data. The Nix flake supplies Go; go.mod/go.sum pin UI dependencies.
# octopus-go-manager
