# Data Model Spec

## Overview

Four tables. One SQLite file. No ORM magic — Drizzle generates SQL, but the schema is designed to be readable as raw SQL.

```
contacts    ─┐
companies   ─┼── core entities (CRUD, merge, import/export)
deals       ─┘
activities  ──── append-only event log
```

## Schema

### contacts

```sql
CREATE TABLE contacts (
  id          TEXT PRIMARY KEY,    -- ULID, prefixed: ct_01J8ZVXB3K...
  name        TEXT NOT NULL,
  emails      TEXT,                -- JSON string[]. E.g. '["jane@acme.com","jane@gmail.com"]'
  phones      TEXT,                -- JSON string[]. Stored as E.164: '["+12125551234"]'
  companies   TEXT,                -- JSON string[]. Company IDs: '["co_01J8Z..."]'
  linkedin    TEXT UNIQUE,         -- Handle only, never URL. E.g. 'janedoe'
  x           TEXT UNIQUE,         -- Handle only. E.g. 'janedoe'
  bluesky     TEXT UNIQUE,         -- Handle. E.g. 'janedoe.bsky.social'
  telegram    TEXT UNIQUE,         -- Handle. E.g. 'janedoe'
  tags        TEXT,                -- JSON string[]. E.g. '["hot-lead","enterprise"]'
  custom      TEXT,                -- JSON object. E.g. '{"title":"CTO","source":"conference"}'
  created_at  TEXT NOT NULL,       -- ISO 8601
  updated_at  TEXT NOT NULL        -- ISO 8601
);
```

### companies

```sql
CREATE TABLE companies (
  id          TEXT PRIMARY KEY,    -- ULID, prefixed: co_01J8ZVXB3K...
  name        TEXT NOT NULL,
  websites    TEXT,                -- JSON string[]. Normalized. E.g. '["acme.com","acme.co.uk"]'
  phones      TEXT,                -- JSON string[]. E.164.
  tags        TEXT,                -- JSON string[]
  custom      TEXT,                -- JSON object
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);
```

### deals

```sql
CREATE TABLE deals (
  id              TEXT PRIMARY KEY,  -- ULID, prefixed: dl_01J8ZVXB3K...
  title           TEXT NOT NULL,
  company         TEXT,              -- Scalar FK → companies.id. Nullable.
  contacts        TEXT,              -- JSON string[]. Contact IDs. Multi-stakeholder.
  value           REAL,
  currency        TEXT DEFAULT 'USD',
  stage           TEXT DEFAULT 'lead',
  pipeline        TEXT DEFAULT 'default',
  expected_close  TEXT,              -- YYYY-MM-DD
  probability     REAL,             -- 0-100
  tags            TEXT,              -- JSON string[]
  custom          TEXT,              -- JSON object
  created_at      TEXT NOT NULL,
  updated_at      TEXT NOT NULL,
  FOREIGN KEY (company) REFERENCES companies(id)
);
```

### activities

```sql
CREATE TABLE activities (
  id          TEXT PRIMARY KEY,    -- ULID, prefixed: ac_01J8ZVXB3K...
  type        TEXT NOT NULL,       -- 'note' | 'call' | 'email' | 'meeting' | 'stage-change'
  contact     TEXT,                -- FK → contacts.id
  company     TEXT,                -- FK → companies.id
  deal        TEXT,                -- FK → deals.id
  body        TEXT,
  custom      TEXT,                -- JSON object. E.g. '{"duration":"15m"}'
  created_at  TEXT NOT NULL,       -- ISO 8601. Immutable.
  FOREIGN KEY (contact) REFERENCES contacts(id),
  FOREIGN KEY (company) REFERENCES companies(id),
  FOREIGN KEY (deal)    REFERENCES deals(id)
);
-- No updated_at. Activities are append-only. Delete and recreate to correct.
```

## Design Decisions

### Why JSON columns instead of junction tables

Contacts have multiple emails, phones, companies, and tags. Companies have multiple websites and phones. Deals link to multiple contacts. The relational way is junction tables (`contact_emails`, `contact_companies`, `deal_contacts`, etc.).

We use JSON columns instead. Here's why:

1. **Scale assumption:** crm.cli targets individual developers or small teams. Expected record counts: <5,000 contacts, <1,000 companies, <500 deals. At this scale, `JSON_EACH()` for querying JSON arrays is fast enough — we're never doing full table scans on 100K rows.

2. **Simpler queries:** `SELECT * FROM contacts WHERE id = ?` returns the complete contact in one row. No joins needed for basic CRUD. This matters because every CLI command is a cold start — no connection pool, no persistent process. Minimizing queries per invocation keeps things snappy.

3. **Simpler code:** One table = one Drizzle schema = one TypeScript type. Junction tables would double the number of schema definitions, migrations, and insert/update operations.

4. **Trade-off acknowledged:** JSON columns can't have SQL-level indexes. You can't do `SELECT * FROM contacts WHERE emails LIKE '%@acme.com'` efficiently (it's a full scan through JSON). For v0.1 at <5K records, this is fine. If the project grows to enterprise scale (10K+ contacts), the migration path is clear: add junction tables, backfill from JSON, keep JSON as a denormalized cache.

### Why JSON TEXT, not SQLite JSON type

SQLite's `JSON` type is really just `TEXT` with a `CHECK(json_valid(column))` constraint. We skip the constraint because:

1. Drizzle ORM handles serialization/deserialization — invalid JSON never hits the DB
2. The Zod validation layer catches malformed input before it reaches Drizzle
3. The constraint adds overhead on every write for zero practical benefit

### Why ULID, not UUID or autoincrement

ULIDs are:
- **Time-sorted:** Records sort chronologically by default. `ORDER BY id` = `ORDER BY created_at`.
- **Collision-resistant:** 80 bits of randomness after the timestamp. Safe for concurrent inserts.
- **Readable-ish:** `01J8ZVXB3K...` starts with an obvious timestamp prefix. Easier to spot in logs than a UUID.
- **Prefix-friendly:** Adding `ct_`, `co_`, `dl_`, `ac_` prefixes makes IDs self-describing. You can look at an ID and immediately know it's a contact, company, deal, or activity.

The prefix is part of the stored ID. It's not stripped before storage — `ct_01J8ZVXB3K...` is the actual primary key value.

### Why no pipelines table

Pipeline stages are defined in `crm.toml`:

```toml
[pipeline]
stages = ["lead", "qualified", "proposal", "negotiation", "closed-won", "closed-lost"]
```

Not in the database. Stages are a configuration concern, not a data concern. Reasons:

1. **Stages rarely change.** When they do, it's a deliberate decision, not a database operation.
2. **Validation is config-driven.** `crm deal move --stage invalid` checks against the config, not a DB lookup.
3. **No FK needed.** The `deals.stage` column stores the stage name as text. Config is the source of truth for valid stage names.
4. **Simpler schema.** One less table, one less migration, one less thing to seed.

If we ever need per-pipeline stage definitions (multiple pipelines with different stages), the config format extends naturally:

```toml
[pipeline.sales]
stages = ["lead", "qualified", "proposal", "won", "lost"]

[pipeline.partnerships]
stages = ["intro", "evaluation", "signed"]
```

### Why stage-change is an activity, not a column on deals

When a deal moves from "lead" to "qualified", we don't update a `stage_history` JSON column on the deal. Instead, we create an activity:

```json
{
  "type": "stage-change",
  "deal": "dl_01J8Z...",
  "body": "lead → qualified",
  "created_at": "2026-04-01T10:30:00Z"
}
```

Stage history is reconstructed by querying activities:

```sql
SELECT body, created_at FROM activities
WHERE deal = ? AND type = 'stage-change'
ORDER BY created_at ASC
```

Why:
1. **Single source of truth.** Activities already track everything that happens. Stage changes are just another event.
2. **Timestamps for free.** Every activity has `created_at`. We get deal velocity (time per stage) without any extra schema.
3. **Append-only.** No race conditions — you never update a JSON array, you only insert a new row.
4. **Reports use it.** Conversion rates, velocity, and forecast reports all query stage-change activities. Having them in the same table as other activities simplifies the reporting queries.

### Why deals.contacts is a JSON array, not a scalar FK

Deals often involve multiple people — the decision-maker, the champion, the technical evaluator. Storing a single `contact_id` forces users to pick one. A JSON array of contact IDs supports the reality of multi-stakeholder deals.

The `--contact` flag is repeatable:

```bash
crm deal add --title "Big Deal" --contact jane@acme.com --contact bob@acme.com
```

`--add-contact` and `--rm-contact` on `deal edit` manage the list.

**FK enforcement note:** `PRAGMA foreign_keys = ON` enforces the scalar FK `deals.company → companies.id`. But JSON array references (`deals.contacts`, `contacts.companies`) are maintained by application code — SQLite FKs don't work on JSON arrays. Deleting a contact runs application code to remove their ID from any deal's `contacts[]` array.

### Why activities are append-only

Activities have no `updated_at` column. Once created, they're immutable. To fix a mistake, delete the activity and create a new one.

This is a deliberate simplification:
1. **Audit trail integrity.** If you can edit activities, you can retroactively change history. Append-only preserves the true record.
2. **Simpler conflict model.** No optimistic concurrency, no "who edited last" problems.
3. **Stage-change activities must be immutable** for velocity/conversion reports to be trustworthy. Making all activities immutable is simpler than making only stage-changes immutable.

### Why custom fields are a flat JSON object

```json
{"title": "CTO", "source": "conference", "score": 85}
```

Not typed columns. Not nested objects. Flat key-value pairs where values are strings in v0.1.

1. **Zero-friction.** `--set title=CTO` on any entity. No migration, no schema change, no "create custom field" step.
2. **FTS5 indexed.** Custom field values are included in the full-text search index. `crm search "CTO"` finds contacts with `title=CTO` in custom fields.
3. **Filterable.** `--filter "title~=CTO"` works on custom fields with the same syntax as core fields.
4. **JSON-typed values via prefix.** `--set "json:score=85"` stores the number `85`, not the string `"85"`. This is a convenience — the JSON object can hold any valid JSON value.

### Referential integrity summary

| Relationship | Type | Enforcement |
|-------------|------|-------------|
| deals.company → companies.id | Scalar FK | SQLite `FOREIGN KEY` constraint |
| activities.contact → contacts.id | Scalar FK | SQLite `FOREIGN KEY` constraint |
| activities.company → companies.id | Scalar FK | SQLite `FOREIGN KEY` constraint |
| activities.deal → deals.id | Scalar FK | SQLite `FOREIGN KEY` constraint |
| contacts.companies[] → companies.id | JSON array | Application code |
| deals.contacts[] → contacts.id | JSON array | Application code |

**Delete behavior:**

| Entity deleted | What happens |
|---------------|-------------|
| Contact | Removed from all `deals.contacts[]` arrays. Activities with `contact = <id>` are kept (orphaned ID is informational). |
| Company | Removed from all `contacts.companies[]` arrays. `deals.company` set to `NULL`. Activities kept. |
| Deal | Activities kept (orphaned deal ID is informational). |

**Merge behavior:** All references are relinked to the surviving entity before the absorbed entity is deleted. No cascades are triggered during merge.
