# FUSE Virtual Filesystem — Implementation Spec

## Why FUSE

FUSE is the core differentiator of crm.cli. Every other CLI CRM stores data in SQLite and exposes it via subcommands. crm.cli does that too — but it also mounts the entire CRM as a virtual filesystem.

This matters because the filesystem is the universal API. Any tool that reads files — `cat`, `grep`, `jq`, `find`, Claude Code, Codex, vim, rsync — instantly has full CRM access. No integration, no MCP server, no SDK. An AI agent with filesystem access can browse contacts, read deals, check the pipeline, and search — all by reading files.

The closest competitor (crmcli.sh) solves AI access via MCP, which requires agents to speak a specific protocol. FUSE solves it at a lower, more universal layer.

## Smoke Test Results (2026-04-04)

### Environment

| Component | Version |
|-----------|---------|
| OS | Debian (Linux 6.1.158) |
| FUSE | fuse3 3.17.2 |
| libfuse3 | 3.17.2-3 (libfuse3-4 + libfuse3-dev) |
| Bun | 1.3.11 |
| Node.js | v20.19.2 |
| gcc | Debian default |

### What we tried

#### Approach 1: N-API bindings via `fuse-native` — FAILED

```bash
bun add fuse-native@2.2.6
bun pm trust fuse-native  # triggers node-gyp rebuild
```

**Failure:** The package bundles node-gyp v6.1.0, which has a known bug: `Cannot assign to read only property 'cflags'`. This is a years-old regression in node-gyp <v9 when running on Node.js 20+. The library was last published ~5 years ago. Dead.

#### Approach 2: N-API bindings via `node-fuse-bindings` — FAILED

```bash
bun add node-fuse-bindings@2.12.4
```

Same bundled node-gyp v6.1.0 bug. Upgrading to global node-gyp v12.2.0 bypasses the first error, but the C++ source targets the FUSE2 API (`fuse.h`, `fuse_operations` without FUSE3 fields). Since the system only has `fuse3`, it fails to compile:

```
Package 'fuse', required by 'virtual:world', not found
```

Symlinking `fuse3.pc` as `fuse.pc` to trick pkg-config gets past discovery, but the actual C++ code uses FUSE2 structs and functions — compile errors on FUSE3 headers.

**Both N-API packages are dead for FUSE3 environments.**

#### Approach 3: Compiled C helper + Bun.spawn() — PASSED

Write a minimal FUSE3 program in C, compile it against libfuse3, spawn it from Bun.

### Reproducing the smoke test

Prerequisites:

```bash
# Install FUSE3 (Debian/Ubuntu)
sudo apt-get install -y fuse3 libfuse3-dev

# Verify
pkg-config --libs fuse3    # should print: -lfuse3 -lpthread
ls /dev/fuse               # should exist
```

If `/dev/fuse` is `0600 root:root`, fix permissions:

```bash
sudo chmod 666 /dev/fuse
# Or add your user to the fuse group:
# sudo usermod -aG fuse $USER
```

Compile the smoke test:

```bash
cd test/fuse-smoke
gcc -Wall -o hello_fuse hello_fuse.c $(pkg-config --cflags --libs fuse3)
```

Run from Bun:

```bash
bun run test/fuse-smoke/smoke.ts
```

Expected output:

```
[smoke] Starting FUSE3 mount...
  ✓ mount point exists
  ✓ readdir returns hello.txt
  ✓ read hello.txt returns JSON
  ✓ nonexistent file throws

[smoke] 4 passed, 0 failed
```

Manual test (shell):

```bash
mkdir -p test/fuse-smoke/mnt
./test/fuse-smoke/hello_fuse -f test/fuse-smoke/mnt &
sleep 1

ls test/fuse-smoke/mnt/                    # should show hello.txt
cat test/fuse-smoke/mnt/hello.txt          # should print JSON
cat test/fuse-smoke/mnt/nonexistent.txt    # should fail ENOENT

fusermount -u test/fuse-smoke/mnt
```

### What the smoke test validates

| Assertion | Status | What it proves |
|-----------|--------|----------------|
| gcc compiles against FUSE3 headers | PASS | libfuse3-dev is sufficient, no special flags needed |
| FUSE mount in foreground mode (`-f`) | PASS | `/dev/fuse` works, kernel module is loaded |
| `readdirSync()` from Bun reads entries | PASS | Bun's Node.js compat layer works with FUSE mounts |
| `readFileSync()` from Bun reads content | PASS | File I/O through FUSE works end-to-end |
| ENOENT for nonexistent files | PASS | Error propagation from FUSE → kernel → Bun is correct |
| `fusermount -u` unmounts cleanly | PASS | Clean teardown path works |
| `proc.kill()` cleans up the FUSE process | PASS | Bun process management is compatible |

### What the smoke test does NOT validate

- SQLite reads from within a FUSE callback (the real implementation reads the DB)
- Symlink resolution (the real implementation uses symlinks for `_by-*` indexes)
- Write operations (deferred to v0.2 in the FUSE layer; CLI handles writes in v0.1)
- Performance under load (hundreds of entities)
- macOS + macFUSE compatibility (only tested on Linux)
- Concurrent mount + CLI access (FUSE reads while CLI writes)

## Implementation Strategy

### Architecture: Compiled C helper binary

The FUSE filesystem is implemented as a separate C binary (`crm-fuse`) that:

1. Receives the SQLite database path and mount point as arguments
2. Opens the database read-only
3. Implements FUSE3 operations (`getattr`, `readdir`, `readlink`, `open`, `read`)
4. Serves entity files as JSON by querying SQLite on each read
5. Serves symlink indexes by querying relationship tables

```
┌──────────┐     spawn      ┌──────────┐     libfuse3     ┌────────┐
│ crm CLI  │ ──────────────→│ crm-fuse │ ←──────────────→ │ kernel │
│  (Bun)   │                │   (C)    │                   │ FUSE   │
└──────────┘                └────┬─────┘                   └────────┘
                                 │
                           ┌─────┴─────┐
                           │  SQLite   │
                           │  (r/o)    │
                           └───────────┘
```

### Why C, not Bun FFI

Bun FFI can call C functions, but FUSE requires registering callbacks — C function pointers passed in a struct to `fuse_main()`. Bun's FFI callback support exists but is limited:

- Each callback must be registered individually via `FFIFunction`
- The FUSE `fuse_operations` struct has ~40 fields, many are complex (buffer pointers, offset arithmetic)
- Lifetime management: FUSE callbacks are called from kernel threads, not the Bun event loop
- Debugging C-level crashes in FFI callbacks is painful

A compiled C binary is simpler, faster, and easier to debug. The tradeoff is a build step — but users installing from npm get the binary via the install script, and developers compile it once via `make`.

### Why not a Node.js subprocess

Running a Node.js/Bun child process that uses an N-API FUSE binding would work in theory, but:

1. **No working N-API FUSE3 binding exists** (see smoke test failures above)
2. Even if one existed, spawning a full Bun runtime for the FUSE daemon adds ~50MB of memory overhead
3. A C binary is ~20KB and starts instantly

### File naming conventions

Entity files use slugified display names with the ID prefix for uniqueness:

```
contacts/
  ct_01J8Z...jane-doe.json        # id prefix + slugified name
  ct_02K9A...john-smith.json
```

This means `ls contacts/` is human-scannable (you see names) but also unique (IDs prevent collisions). If two contacts are both named "Jane Doe", they get `ct_01J...jane-doe.json` and `ct_02K...jane-doe.json` — the IDs differentiate them.

### Symlink index structure

Each `_by-*` directory contains symlinks pointing back to canonical entity files:

```
_by-email/
  jane@acme.com.json → ../ct_01J8Z...jane-doe.json        # one level up
_by-company/
  acme-corp/
    ct_01J8Z...jane-doe.json → ../../ct_01J8Z...jane-doe.json  # two levels up
```

Depth rule: flat indexes (`_by-email/`, `_by-phone/`) use `../`. Grouped indexes (`_by-company/<name>/`, `_by-tag/<tag>/`) use `../../`.

A contact with multiple emails gets one symlink per email. A contact in multiple companies appears under each company's subdirectory.

### Process lifecycle

```
crm mount ~/crm [--db path] [--readonly]
  1. Resolve DB path (flag > env > config > default)
  2. Check if crm-fuse binary exists; if not, print install instructions
  3. Check if mount point exists; create if needed
  4. Check if already mounted (read /proc/mounts or mount output)
  5. Spawn: crm-fuse --db <path> --mountpoint <dir> [-f]
  6. Write PID to ~/.crm/mount.pid
  7. Wait for mount to become available (poll readdir on mount point)
  8. Print "Mounted at ~/crm"

crm unmount [~/crm]
  1. Read PID from ~/.crm/mount.pid
  2. Run: fusermount -u <mountpoint>
  3. Kill process if still alive
  4. Remove PID file
```

### Platform support

| Platform | FUSE library | Status |
|----------|-------------|--------|
| Linux (x64, arm64) | libfuse3 (`apt install fuse3 libfuse3-dev`) | Primary target. Smoke-tested. |
| macOS (Intel, Apple Silicon) | macFUSE (`brew install macfuse`) | Planned. macFUSE uses FUSE2 API with a FUSE3 compat layer — needs testing. Kernel extension required; macOS 11+ may need SIP adjustment. |
| macOS | FUSE-T (`brew install fuse-t`) | Alternative to macFUSE. Pure userspace, no kernel extension. Newer, less battle-tested. Worth evaluating. |
| Windows | N/A | Not supported. Use WSL. |
| Containers / sandboxes | Depends on `/dev/fuse` access | Requires `--privileged` or `--device /dev/fuse`. Falls back to CLI with `--format json` if unavailable. |

### Fallback: `crm export-fs`

If FUSE is unavailable (no libfuse, container without `/dev/fuse`, Windows without WSL), a static export provides the same file structure without the live mount:

```bash
crm export-fs ~/crm-export
```

This generates the full directory tree (entities as JSON files, symlink indexes) as a one-time snapshot. Not live — stale until re-run. But covers the core use case of "AI agent reads CRM files" without any kernel dependency.

## Build and distribution

### Compiling crm-fuse

```bash
# Linux
gcc -O2 -Wall -o crm-fuse src/fuse-helper.c \
  $(pkg-config --cflags --libs fuse3) \
  $(pkg-config --cflags --libs sqlite3)

# macOS (with macFUSE)
clang -O2 -Wall -o crm-fuse src/fuse-helper.c \
  $(pkg-config --cflags --libs fuse) \
  $(pkg-config --cflags --libs sqlite3)
```

### Install script integration

The `install.sh` script with `--all` flag:

1. Downloads the pre-compiled `crm` binary (Bun-compiled)
2. Downloads the pre-compiled `crm-fuse` binary for the platform
3. Checks for libfuse3/macFUSE; prints install instructions if missing
4. Places both binaries in `~/.local/bin`

### CI/CD

GitHub Actions builds `crm-fuse` for all supported platforms on each release tag:

```yaml
# Simplified — cross-compile matrix
strategy:
  matrix:
    include:
      - os: ubuntu-latest
        target: linux-x64
        cc: gcc
      - os: ubuntu-latest  # cross-compile via gcc-aarch64-linux-gnu
        target: linux-arm64
        cc: aarch64-linux-gnu-gcc
      - os: macos-13       # Intel runner
        target: darwin-x64
        cc: clang
      - os: macos-14       # Apple Silicon runner
        target: darwin-arm64
        cc: clang
```

## Open questions

1. **macFUSE FUSE2 vs FUSE3 API:** macFUSE exposes a FUSE2-compatible API. The `crm-fuse` helper targets FUSE3 (`#define FUSE_USE_VERSION 35`). Need to test if macFUSE's compat layer is sufficient or if we need a FUSE2 code path (conditional compilation via `#ifdef`).

2. **SQLite from C vs from Bun:** The FUSE helper reads SQLite directly via the C API. This means SQLite queries are duplicated — once in the Drizzle ORM (for CLI commands) and once in raw C SQL (for FUSE reads). Keeping these in sync is a maintenance burden. Alternative: have the FUSE helper call the Bun CLI (`crm show <id> --format json`) on each read — simpler but slower.

3. **Cache invalidation:** If the FUSE mount caches query results, CLI writes won't be reflected until the cache expires. Options: no cache (query on every read — simple, works for <5K records), inotify on the SQLite file, or a shared-memory signal from the CLI to the FUSE daemon.

4. **Concurrent access:** SQLite with WAL mode supports concurrent readers. The FUSE helper opens the DB read-only, the CLI opens read-write. This should work — but needs testing under load (many rapid CLI writes while FUSE is serving reads).
