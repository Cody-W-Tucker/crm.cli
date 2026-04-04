# Normalization Contracts

## Philosophy

crm.cli normalizes input data on write so that lookups, deduplication, and display work consistently regardless of how the user typed it. This is the structured intelligence that justifies using a CRM over a spreadsheet.

Three normalization tiers:

| Tier | Fields | Library | Invalid input |
|------|--------|---------|---------------|
| **Strict** | Phone numbers | libphonenumber-js | Reject with error |
| **Permissive** | Websites | normalize-url | Store as-is if normalization fails |
| **Extract** | Social handles | Custom regex | Store as-is if no pattern matches |

The rule: phones are strict because a wrong E.164 value corrupts lookups permanently. Websites and handles are permissive because storing a weird URL is better than rejecting potentially valid input.

## Phone Normalization

### Library

[libphonenumber-js](https://gitlab.com/nicolo-ribaudo/libphonenumber-js) — Google's libphonenumber ported to JavaScript. Battle-tested, handles edge cases across all country formats.

### Behavior

**Storage format:** E.164 (`+12125551234`). Always. No exceptions.

**Input parsing:** Any reasonable format is accepted:

| Input | Stored as | Notes |
|-------|-----------|-------|
| `+1-212-555-1234` | `+12125551234` | International with dashes |
| `(212) 555-1234` | `+12125551234` | National (requires `default_country`) |
| `2125551234` | `+12125551234` | Digits only (requires `default_country`) |
| `+12125551234` | `+12125551234` | Already E.164 |
| `+44 20 7946 0958` | `+442079460958` | UK number |
| `020 7946 0958` | `+442079460958` | UK national (with `default_country = "GB"`) |

**Validation rules:**
- Must be a valid phone number per libphonenumber-js `isValidNumber()`
- Must have enough digits for the country
- Country code required unless `phone.default_country` is set in config
- Invalid numbers → error exit (non-zero), stderr message: `error: invalid phone number "<input>"`

**Config:**

```toml
[phone]
default_country = "US"           # ISO 3166-1 alpha-2
display = "international"        # international | national | e164
```

**Display formats:**

| Setting | Output for +12125551234 |
|---------|------------------------|
| `international` | `+1 212 555 1234` |
| `national` | `(212) 555-1234` |
| `e164` | `+12125551234` |

Display format only affects `show` and `list` output. Storage is always E.164. JSON output (`--format json`) always uses E.164.

**Lookup:** Any format resolves to E.164 before querying:

```bash
crm contact show "+12125551234"      # E.164
crm contact show "(212) 555-1234"    # national → E.164 → lookup
crm contact show "212-555-1234"      # partial → E.164 → lookup
# All three find the same contact
```

**Deduplication:** Adding a phone that normalizes to an E.164 already in the database is rejected as a duplicate.

### Edge cases (covered by tests)

- Numbers without country code and no `default_country` config → error
- Numbers with too few digits → error
- Numbers with letters → error
- Toll-free numbers (e.g., `+1-800-555-1234`) → valid, normalized
- Numbers with extensions → stripped (extensions not stored in v0.1)
- Leading/trailing whitespace → stripped before parsing

## Website Normalization

### Library

[normalize-url](https://github.com/sindresorhus/normalize-url) — well-maintained, covers the common normalization cases.

### Behavior

**Storage format:** Lowercase host, no protocol, no `www.`, preserved path, no trailing slash (unless path is non-empty).

| Input | Stored as |
|-------|-----------|
| `https://www.Acme.com` | `acme.com` |
| `http://acme.com/` | `acme.com` |
| `HTTPS://WWW.ACME.COM/Labs` | `acme.com/labs` |
| `acme.com/labs/` | `acme.com/labs` |
| `acme.com` | `acme.com` |
| `https://us.acme.com` | `us.acme.com` |
| `http://acme.co.uk` | `acme.co.uk` |

**Normalization rules:**
1. Strip protocol (`http://`, `https://`)
2. Strip `www.` prefix
3. Lowercase the entire host
4. Preserve the path (case-sensitive — paths can be case-sensitive on servers)
5. Strip trailing slash only when path is `/` (root)
6. Strip query string and fragment

**Invalid input:** If `normalize-url` throws, store the input as-is. This is the permissive tier — we'd rather store `not-a-url.example` than reject it.

**Uniqueness:**
- Same normalized website on two different companies → rejected as duplicate
- Different paths are distinct: `globex.com/research` ≠ `globex.com/consulting`
- Different subdomains are distinct: `us.acme.com` ≠ `eu.acme.com`
- `www.` is stripped, so `www.acme.com` = `acme.com`

**Lookup:** Input is normalized before querying. `crm company show "HTTPS://WWW.ACME.COM"` finds the company stored as `acme.com`.

### Edge cases (covered by tests)

- Protocol-only input (`https://`) → stored as-is (normalization fails gracefully)
- IP addresses (`192.168.1.1`) → stored as-is
- Ports (`acme.com:8080`) → preserved
- Unicode domains (IDN) → stored as-is (no punycode conversion in v0.1)
- Multiple consecutive slashes in path → collapsed
- Query strings → stripped
- Fragments (`#section`) → stripped
- Data URIs, `javascript:` → stored as-is (permissive)
- Empty string → error (website is a non-empty field when provided)

## Social Handle Normalization

### Platforms

Four hard-coded social platforms with handle fields on contacts:

| Platform | Column | Example handle |
|----------|--------|---------------|
| LinkedIn | `linkedin` | `janedoe` |
| X / Twitter | `x` | `janedoe` |
| Bluesky | `bluesky` | `janedoe.bsky.social` |
| Telegram | `telegram` | `janedoe` |

### Behavior

**Storage format:** Handle only. Never a URL, never an `@` prefix.

**Input parsing:** Accept any of these formats and extract the handle:

| Input | Platform | Stored as |
|-------|----------|-----------|
| `janedoe` | Any | `janedoe` |
| `@janedoe` | Any | `janedoe` |
| `https://linkedin.com/in/janedoe` | LinkedIn | `janedoe` |
| `linkedin.com/in/janedoe` | LinkedIn | `janedoe` |
| `www.linkedin.com/in/janedoe` | LinkedIn | `janedoe` |
| `https://linkedin.com/in/janedoe/` | LinkedIn | `janedoe` |
| `x.com/janedoe` | X | `janedoe` |
| `twitter.com/janedoe` | X | `janedoe` |
| `https://x.com/janedoe` | X | `janedoe` |
| `bsky.app/profile/user.bsky.social` | Bluesky | `user.bsky.social` |
| `https://bsky.app/profile/user.bsky.social` | Bluesky | `user.bsky.social` |
| `t.me/janedoe` | Bluesky | `janedoe` |
| `https://t.me/janedoe` | Telegram | `janedoe` |

**Extraction rules per platform:**

```
LinkedIn:
  URL pattern: linkedin.com/in/<handle>[/...]
  Strip: protocol, www., /in/ prefix, trailing slash
  Result: <handle>

X / Twitter:
  URL pattern: (x.com|twitter.com)/<handle>[/...]
  Strip: protocol, www., domain, trailing slash
  Result: <handle>

Bluesky:
  URL pattern: bsky.app/profile/<handle>[/...]
  Strip: protocol, www., /profile/ prefix, trailing slash
  Result: <handle> (includes .bsky.social)

Telegram:
  URL pattern: t.me/<handle>[/...]
  Strip: protocol, www., domain, trailing slash
  Result: <handle>

All platforms:
  If input starts with @, strip the @
  If no URL pattern matches, store as-is (permissive)
```

**Uniqueness:** Each platform's handle column is `UNIQUE` in the schema. No two contacts can have the same LinkedIn handle, the same X handle, etc. Cross-platform duplicates are fine (same handle on LinkedIn and X).

**Lookup:** `crm contact show janedoe` matches against all four social handle columns. `crm contact show linkedin.com/in/janedoe` extracts the handle first, then matches. This means URL input works for lookups too.

**Duplicate detection:** The `crm dupes` command flags contacts with similar handles across platforms as potential duplicates (e.g., `janedoe` on LinkedIn and `janetdoe` on X for two different contacts).

### Why hard-coded, not extensible

Four platforms are hard-coded as columns, not stored in a generic `socials JSON` field. Reasons:

1. **UNIQUE constraints.** SQLite can enforce uniqueness on columns, not on JSON keys.
2. **FUSE indexes.** Each platform gets a `_by-linkedin/`, `_by-x/`, etc. directory. These are defined at the schema level, not dynamically.
3. **URL extraction patterns.** Each platform has a specific URL format. A generic system would need a config-driven pattern registry — over-engineering for 4 platforms.
4. **These four cover 95% of professional networking.** If someone needs GitHub, Mastodon, or Instagram handles, they go in `custom_fields` via `--set github=octocat`.

Adding a fifth platform (e.g., GitHub) is a schema migration + new URL parser + new FUSE index. Straightforward but deliberate — you don't accidentally add social platforms.
