# Test Inventory

## Methodology

All tests are **black-box functional tests**. They exercise the CLI binary via `Bun.spawnSync` — the same way a user would run it. No unit tests, no mocking, no internal imports.

```typescript
// test/helpers.ts — the test harness
const proc = Bun.spawnSync(['bun', 'run', CRM_BIN, '--db', dbPath, ...args])
```

Each test gets a fresh temp directory and database. Tests are isolated — no shared state, no ordering dependencies.

### Test context API

```typescript
const ctx = createTestContext()
// ctx.dir       — temp directory
// ctx.dbPath    — path to fresh SQLite DB
// ctx.run(...)  — run command, return { stdout, stderr, exitCode }
// ctx.runOK(...)   — run command, assert exit 0, return stdout
// ctx.runFail(...) — run command, assert exit != 0, return full result
// ctx.runJSON<T>(...) — run command, parse stdout as JSON, return typed object
// ctx.runWithEnv(env, ...) — run with custom env vars (for config tests)
```

## Test Files

### contact.test.ts — 85 tests

The largest test file. Covers the full lifecycle of contacts.

| Section | Tests | What it covers |
|---------|-------|----------------|
| CRUD basics | ~20 | add, list, show, edit, rm with all flags |
| Multi-value fields | ~15 | --email/--phone/--company repeatable, --add-*/--rm-* on edit |
| Phone normalization | ~12 | E.164 storage, various input formats, invalid rejection, lookup by any format |
| Website normalization | ~8 | On company auto-creation, protocol stripping, www removal, path preservation |
| Social handles | ~15 | LinkedIn/X/Bluesky/Telegram URL extraction, handle storage, lookup by URL, uniqueness |
| Entity resolution | ~8 | Show/edit/rm by email, phone, or social handle (not just ID) |
| Tags | ~4 | --tag on add, --add-tag/--rm-tag on edit |
| Custom fields | ~3 | --set key=value, --unset, filter on custom fields |

### company.test.ts — 45 tests

| Section | Tests | What it covers |
|---------|-------|----------------|
| CRUD basics | ~15 | add, list, show, edit, rm |
| Multi-value fields | ~8 | --website/--phone repeatable, --add-*/--rm-* |
| Website normalization | ~10 | Protocol strip, www strip, path preservation, subdomain distinction, dedup |
| Company merge | ~5 | Merge two companies, relink contacts and deals |
| Auto-creation | ~4 | Company created as stub when referenced by --company on contact add |
| Tags + custom fields | ~3 | Same patterns as contacts |

### deal.test.ts — 39 tests

| Section | Tests | What it covers |
|---------|-------|----------------|
| CRUD basics | ~10 | add, list, show, edit, rm |
| Stage management | ~5 | deal move, stage validation against config, same-stage rejection |
| Stage-change activities | ~3 | Move creates activity with type=stage-change, body contains old→new, timestamp |
| Stage history | ~1 | Reconstructed from activity log entries |
| Multi-contact deals | ~5 | --contact repeatable, --add-contact/--rm-contact on edit, filter by any contact |
| Error paths | ~11 | Nonexistent contact/company refs, invalid probability (>100), negative value, invalid date, show/edit/rm nonexistent deal |
| Referential integrity | ~4 | Delete contact → deal contacts cleared, delete company → deal company null |

### fuse.test.ts — 61 tests

| Section | Tests | What it covers |
|---------|-------|----------------|
| Contact files | ~10 | Read entity JSON, correct fields, linked data inlined |
| Company files | ~6 | Read entity JSON, website normalization in filenames |
| Deal files | ~5 | Read entity JSON, stage in filename path |
| Symlink indexes | ~15 | _by-email, _by-phone, _by-company, _by-tag, _by-linkedin, _by-x, _by-stage |
| Directory listing | ~5 | readdir returns correct entries, proper file/dir types |
| Report files | ~5 | pipeline.json, stale.json, forecast.json, etc. |
| Search via filesystem | ~3 | Read search/<query>.json, results returned as JSON |
| Write operations | ~2 | Create entity via file write, update via full-document replace |
| Error states | ~11 | ENOENT for nonexistent entities/symlinks/subdirs, write to invalid dir, delete from index dirs |

**Note:** FUSE tests use a `skipIfNoFuse()` guard. They pass trivially (skip) if FUSE is not available. Real FUSE testing requires `/dev/fuse` access.

### activity.test.ts — 13 tests

| Section | Tests | What it covers |
|---------|-------|----------------|
| Log commands | ~5 | note, call, meeting, email types with entity refs |
| Activity list | ~4 | Filter by contact, company, deal, type, date range |
| Custom fields | ~2 | --set duration=15m, --at override timestamp |
| Append-only | ~2 | No edit command, activities are immutable |

### search.test.ts — 12 tests

| Section | Tests | What it covers |
|---------|-------|----------------|
| FTS5 keyword search | ~6 | Basic query, type filter, multi-type filter, format json |
| Semantic search (crm find) | ~4 | Natural language query, type filter, threshold, limit |
| Index management | ~2 | crm index rebuild, crm index status |

### report.test.ts — 17 tests

| Section | Tests | What it covers |
|---------|-------|----------------|
| Pipeline report | ~3 | Summary with counts/values/weighted, JSON format, markdown format |
| Activity report | ~3 | Time period, group by type, group by contact |
| Stale report | ~2 | Default 14 days, custom days, type filter |
| Conversion report | ~2 | Stage-to-stage rates, since filter |
| Velocity report | ~2 | Average time per stage, won-only filter |
| Forecast report | ~2 | Weighted by close date, period filter |
| Won/lost reports | ~3 | Won summary, lost summary with reasons, period filter |

### config.test.ts — 12 tests

| Section | Tests | What it covers |
|---------|-------|----------------|
| Config resolution | ~4 | Walk-up from CWD, --config flag, CRM_CONFIG env var, global fallback |
| Pipeline stages | ~3 | Custom stages from config, validation against config stages |
| Phone config | ~2 | default_country, display format |
| Database path | ~2 | Config-specified db path, flag override |
| Format defaults | ~1 | defaults.format in config |

### import-export.test.ts — 19 tests

| Section | Tests | What it covers |
|---------|-------|----------------|
| CSV import | ~4 | Basic import, header mapping, --dry-run, --skip-errors |
| JSON import | ~2 | Array of objects, same flags |
| Export | ~3 | CSV export, JSON export, export all |
| Edge cases | ~7 | Missing columns, extra columns → custom fields, phone normalization on import, duplicate skip, stdin import, empty file, company website normalization |
| Round-trip | ~3 | Export → import produces identical data |

### dupes.test.ts — 13 tests

| Section | Tests | What it covers |
|---------|-------|----------------|
| Company dupes | ~3 | Similar names, similar websites, flagged pairs |
| Contact dupes | ~7 | Similar social handles, shared email domain + similar names, different contacts NOT flagged, threshold, limit, empty data |
| Cross-entity | ~1 | Without --type searches both contacts and companies |
| Output format | ~2 | Table and JSON output with reasons |

### global.test.ts — 9 tests

| Section | Tests | What it covers |
|---------|-------|----------------|
| --version | ~1 | Prints version string |
| --db flag | ~2 | Custom DB path, env var override |
| --format flag | ~2 | Global format flag, env var override |
| --no-color | ~1 | NO_COLOR env var |
| --help | ~2 | Main help, subcommand help |
| Error cases | ~1 | Unknown command |

### tag.test.ts — 8 tests

| Section | Tests | What it covers |
|---------|-------|----------------|
| Tag/untag | ~3 | Add tags, remove tags, multiple tags at once |
| Tag list | ~3 | All tags with counts, filter by entity type |
| Cross-entity | ~2 | Tags on contacts, companies, and deals |

### hook.test.ts — 4 tests

| Section | Tests | What it covers |
|---------|-------|----------------|
| Post hooks | ~2 | Shell command runs after contact-add, receives JSON on stdin |
| Pre hooks | ~1 | Non-zero exit aborts the operation |
| Hook config | ~1 | Hooks configured via crm.toml |

## Coverage Map

```
                    CRUD  Normalize  Merge  Import  Search  FUSE  Report  Hook  Config
contacts             ██     ██        ██      ██      ██     ██
companies            ██     ██        ██      ██      ██     ██
deals                ██                       ██      ██     ██     ██
activities           ██                               ██     ██     ██
tags                 ██                               ██     ██
pipeline                                              ██     ██     ██
config                                                                      ██
hooks                                                                  ██
global flags                                                               ██
dupes                              ██
```

## Implementation Order → Test File Mapping

| Step | Implementation | Test file(s) to make green |
|------|---------------|---------------------------|
| 0 | FUSE smoke test | ~~(manual, not bun test)~~ **DONE** |
| 1 | Schema + migrations | (no dedicated test file — validates via all tests) |
| 2 | Config resolution | config.test.ts (12) |
| 3 | Global flags + help | global.test.ts (9) |
| 4 | Contact CRUD | contact.test.ts (85) — core subset |
| 5 | Normalization layer | contact.test.ts (85) — normalization subset |
| 6 | Company CRUD | company.test.ts (45) |
| 7 | Deal CRUD + pipeline | deal.test.ts (39) |
| 8 | Activity log | activity.test.ts (13) |
| 9 | Tag commands | tag.test.ts (8) |
| 10 | Merge + dupes | dupes.test.ts (13) + merge tests in contact/company |
| 11 | Import/export | import-export.test.ts (19) |
| 12 | FTS5 search | search.test.ts (12) — FTS5 subset |
| 13 | Reports | report.test.ts (17) |
| 14 | Hooks | hook.test.ts (4) |
| 15 | Semantic search | search.test.ts (12) — semantic subset |
| 16 | FUSE mount | fuse.test.ts (61) |

**Total: 337 tests. All currently failing (implementation not started).**

## Running Tests

```bash
# All tests
bun test

# Single file
bun test test/contact.test.ts

# Pattern match
bun test --filter "phone normalization"

# With verbose output
bun test --verbose
```

Tests run fast because each one spawns a fresh process and uses a temp DB. No shared state means tests can run in parallel (Bun's default).
