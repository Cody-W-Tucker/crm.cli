# Architecture

## Design Principles

1. **Spec first, then implement.** The README is the interface spec. Functional tests are the behavioral contracts. Implementation = make tests green. This means all design decisions are made and documented before a single line of implementation code is written.

2. **Every command is a cold start.** No daemon, no persistent process (except the FUSE mount). Each `crm` invocation opens the DB, runs the operation, and exits. This constrains the design: no connection pools, no in-memory caches, no background workers. But it also means the CLI is stateless and predictable.

3. **The filesystem is the API.** FUSE mount is the primary integration surface. Any tool that reads files has CRM access. The CLI is for writes and interactive use; the filesystem is for reads and automation.

4. **SQLite is the entire backend.** No Redis, no Postgres, no ElasticSearch. One `.db` file. This limits scale but eliminates ops. The target user has <5,000 contacts — SQLite handles that without breaking a sweat.

5. **Normalization on write, not read.** Phone numbers are normalized to E.164 when entered, not when displayed. This means storage is canonical — lookups, dedup, and reports all work on normalized data without re-parsing.

## Stack

```
┌─────────────────────────────────────────────────────┐
│                    User / Agent                      │
├──────────────┬──────────────────┬───────────────────┤
│  CLI (Bun)   │   FUSE mount     │  Pipes / jq / etc │
│  commander   │   (C binary)     │                   │
├──────────────┴──────────────────┴───────────────────┤
│                 Validation (Zod)                      │
├─────────────────────────────────────────────────────┤
│              Normalization Layer                      │
│  libphonenumber-js │ normalize-url │ handle extract  │
├─────────────────────────────────────────────────────┤
│                 Drizzle ORM                           │
├─────────────────────────────────────────────────────┤
│                 libSQL (SQLite)                       │
├─────────────────────────────────────────────────────┤
│                 ~/.crm/crm.db                        │
└─────────────────────────────────────────────────────┘
```

### Why Bun

- **Single binary distribution.** `bun build --compile` produces a standalone executable. No Node.js runtime required on the user's machine.
- **Fast startup.** Bun starts in ~25ms vs Node.js ~75ms. Matters because every command is a cold start.
- **Native SQLite.** Bun includes SQLite natively (via `bun:sqlite`). We use libSQL for Drizzle compatibility, but the option exists.
- **Test runner.** `bun test` is built in. No Jest, no Vitest, no config.

### Why libSQL + Drizzle (not bun:sqlite)

Bun's native SQLite (`bun:sqlite`) is fast but doesn't integrate with Drizzle ORM. Drizzle provides:

- Type-safe schema definition
- Migration generation
- Query builder that prevents SQL injection
- Consistent API if we ever need to support remote databases (libSQL supports Turso)

The overhead of libSQL over `bun:sqlite` is negligible for our workload.

### Why commander (not custom arg parsing)

`commander` is the standard CLI framework for Node.js/Bun:

- Handles subcommands, flags, help text, validation
- Supports repeatable flags (`--email` multiple times)
- Well-documented, stable, zero surprises
- The alternative (custom `process.argv` parsing) is a maintenance burden for ~30 subcommands

### Why Zod

Input validation layer between user input and the database:

- Validates flag values before they reach Drizzle
- Validates JSON input for import and FUSE writes
- Generates clear error messages (`"expected string, got number"`)
- Type inference: Zod schemas generate TypeScript types, so the validator and the type system are always in sync

## Directory Structure (planned)

```
crm.cli/
├── src/
│   ├── cli.ts              # Entry point. commander setup, subcommand routing
│   ├── db/
│   │   ├── schema.ts       # Drizzle schema (4 tables)
│   │   ├── migrate.ts      # Migration runner
│   │   └── connection.ts   # DB connection factory (open, close, WAL mode)
│   ├── commands/
│   │   ├── contact.ts      # contact add/list/show/edit/rm/merge
│   │   ├── company.ts      # company add/list/show/edit/rm/merge
│   │   ├── deal.ts         # deal add/list/show/edit/rm/move
│   │   ├── activity.ts     # log, activity list
│   │   ├── tag.ts          # tag, untag, tag list
│   │   ├── search.ts       # search (FTS5), find (semantic), dupes
│   │   ├── report.ts       # pipeline, activity, stale, conversion, velocity, forecast, won/lost
│   │   ├── import-export.ts
│   │   ├── pipeline.ts     # pipeline summary
│   │   ├── mount.ts        # FUSE mount/unmount (spawns crm-fuse binary)
│   │   ├── config.ts       # config resolution
│   │   └── index.ts        # re-export all commands, search index rebuild
│   ├── normalize/
│   │   ├── phone.ts        # E.164 via libphonenumber-js
│   │   ├── website.ts      # via normalize-url
│   │   └── social.ts       # handle extraction (LinkedIn, X, Bluesky, Telegram)
│   ├── format/
│   │   ├── table.ts        # ASCII table formatter
│   │   ├── json.ts         # JSON output
│   │   └── csv.ts          # CSV/TSV output
│   ├── hooks.ts            # Pre/post hook execution
│   ├── fuse-helper.c       # FUSE3 filesystem implementation (compiled separately)
│   └── search/
│       ├── fts5.ts         # FTS5 index management
│       └── semantic.ts     # ONNX embedding model (optional)
├── spec/                   # This directory — design specs
├── test/
│   ├── helpers.ts          # createTestContext(), runOK(), runFail(), runJSON()
│   ├── contact.test.ts     # 85 tests
│   ├── company.test.ts     # 45 tests
│   ├── deal.test.ts        # 39 tests
│   ├── fuse.test.ts        # 61 tests
│   ├── activity.test.ts    # 13 tests
│   ├── search.test.ts      # 12 tests
│   ├── report.test.ts      # 17 tests
│   ├── config.test.ts      # 12 tests
│   ├── import-export.test.ts # 19 tests
│   ├── dupes.test.ts       # 13 tests
│   ├── global.test.ts      # 9 tests
│   ├── tag.test.ts         # 8 tests
│   ├── hook.test.ts        # 4 tests
│   └── fuse-smoke/         # FUSE3 smoke test (C binary + Bun runner)
└── install.sh              # Platform-aware installer
```

## Command Flow

Every CLI command follows the same flow:

```
1. Parse args (commander)
2. Resolve config (crm.toml walk-up, env vars, flags)
3. Open DB connection (libSQL, WAL mode, foreign keys ON)
4. Run pre-hook (if configured)
5. Validate input (Zod)
6. Normalize input (phone/website/social)
7. Execute operation (Drizzle query)
8. Update FTS5 index (if write operation)
9. Run post-hook (if configured)
10. Format output (table/json/csv/tsv/ids)
11. Print to stdout (data) or stderr (errors)
12. Exit 0 (success) or 1 (error)
```

**Error handling:** Errors go to stderr. Data goes to stdout. Exit code 0 = success, 1 = user error (invalid input, not found), 2 = system error (DB locked, disk full). This follows Unix conventions and makes pipe-friendly error handling possible.

## Config Resolution

Config is loaded from `crm.toml`. Resolution order (first match wins):

```
1. --config <path> flag
2. CRM_CONFIG env var
3. Walk up from CWD: ./crm.toml → ../crm.toml → ../../crm.toml → ... → /crm.toml
4. ~/.crm/config.toml (global fallback)
```

This mirrors `.gitignore`, `tsconfig.json`, and other config-walk patterns. A project-level `crm.toml` overrides global config, so teams can share pipeline stages and defaults.

## Search Architecture

### FTS5 (keyword search)

SQLite's built-in full-text search. A virtual table (`crm_fts`) indexes:

- Contact: name, emails, phones, companies, tags, custom field values
- Company: name, websites, phones, tags, custom field values
- Deal: title, stage, tags, custom field values
- Activity: body, type

Index updates happen synchronously on every write. `crm index rebuild` rebuilds from scratch.

### Semantic search (optional)

`crm find` uses a local ONNX embedding model (`all-MiniLM-L6-v2`, ~80MB). The model downloads on first use and caches at `~/.crm/models/`.

Embedding workflow:
1. On write: generate embedding vector for the entity's text representation
2. Store vector in a separate table (`embeddings`)
3. On search: embed the query, compute cosine similarity against all vectors
4. Return top-K results above threshold

**Dependency:** `onnxruntime-node` (~50MB). Optional — `crm find` falls back to FTS5 keyword search if ONNX is unavailable. The install script's `--all` flag includes ONNX; `--minimal` skips it.

**Why local, not API:** No API keys, no network calls, no data leaving the machine. The target user is a developer who cares about privacy and offline capability.

## Distribution

### Three install paths

1. **npm/bun global install:** `bun install -g crm.cli` or `npm install -g crm.cli`. The `bin` field in `package.json` points to `src/cli.ts`, which Bun runs directly.

2. **Compiled binary:** `bun build --compile` produces a standalone executable for each platform. Distributed via GitHub Releases. No runtime dependency (Bun is embedded in the binary).

3. **Install script:** `install.sh` detects platform, downloads the correct binary, installs to `~/.local/bin`. Options:
   - `--all`: installs binary + FUSE helper + ONNX runtime
   - `--minimal`: binary only (no FUSE, no semantic search)

### FUSE helper distribution

The FUSE helper (`crm-fuse`) is a separate C binary. It's not bundled inside the Bun-compiled executable — it's distributed alongside it.

- **GitHub Releases:** Pre-compiled `crm-fuse` for each platform (linux-x64, linux-arm64, darwin-x64, darwin-arm64)
- **Install script:** Downloads and installs both `crm` and `crm-fuse`
- **From source:** `make fuse` compiles `src/fuse-helper.c` against the system's libfuse3

If `crm-fuse` is missing, `crm mount` prints platform-specific install instructions and exits with an error. All other CLI commands work without it.

## Concurrency Model

- **CLI operations:** Single-threaded, synchronous. One command at a time. SQLite's WAL mode handles concurrent readers.
- **FUSE reads + CLI writes:** The FUSE helper opens the DB read-only. The CLI opens read-write. WAL mode allows this concurrently. The FUSE helper queries on every read (no cache), so CLI writes are reflected immediately on the next FUSE read.
- **Multiple CLIs:** Two terminal sessions can run `crm` simultaneously. SQLite WAL handles concurrent writes with a brief lock wait (default 5s timeout). If the lock times out, the second writer gets "database locked" error.
