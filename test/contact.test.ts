import { describe, expect, test } from 'bun:test'

import { createTestContext } from './helpers.ts'

describe('contact add', () => {
  test('basic add returns prefixed ID', () => {
    const ctx = createTestContext()
    const out = ctx.runOK('contact', 'add', '--name', 'Jane Doe')
    expect(out.trim()).toStartWith('ct_')
  })

  test('full add stores all fields', () => {
    const ctx = createTestContext()
    const id = ctx
      .runOK(
        'contact', 'add',
        '--name', 'Jane Doe',
        '--email', 'jane@acme.com',
        '--phone', '+1-555-0100',
        '--company', 'Acme Corp',
        '--title', 'CTO',
        '--source', 'conference',
        '--tag', 'hot-lead',
        '--tag', 'enterprise',
        '--set', 'linkedin=linkedin.com/in/janedoe',
      )
      .trim()

    const show = ctx.runOK('contact', 'show', id)
    expect(show).toContain('Jane Doe')
    expect(show).toContain('jane@acme.com')
    expect(show).toContain('+1-555-0100')
    expect(show).toContain('CTO')
    expect(show).toContain('conference')
    expect(show).toContain('hot-lead')
    expect(show).toContain('enterprise')
    expect(show).toContain('linkedin.com/in/janedoe')
  })

  test('fails without --name', () => {
    const ctx = createTestContext()
    const result = ctx.runFail('contact', 'add', '--email', 'nobody@example.com')
    expect(result.stderr).toContain('name')
  })

  test('rejects duplicate email', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'Jane Doe', '--email', 'jane@acme.com')
    const result = ctx.runFail('contact', 'add', '--name', 'Jane Smith', '--email', 'jane@acme.com')
    expect(result.stderr).toContain('duplicate')
  })

  test('multiple emails on create', () => {
    const ctx = createTestContext()
    const id = ctx.runOK(
      'contact', 'add', '--name', 'Jane Doe',
      '--email', 'jane@acme.com', '--email', 'jane.doe@gmail.com',
    ).trim()

    const show = ctx.runOK('contact', 'show', id)
    expect(show).toContain('jane@acme.com')
    expect(show).toContain('jane.doe@gmail.com')
  })

  test('multiple phones on create', () => {
    const ctx = createTestContext()
    const id = ctx.runOK(
      'contact', 'add', '--name', 'Jane Doe',
      '--phone', '+1-555-0100', '--phone', '+44-20-7946-0958',
    ).trim()

    const show = ctx.runOK('contact', 'show', id)
    expect(show).toContain('+1-555-0100')
    expect(show).toContain('+44-20-7946-0958')
  })

  test('lookup by any email when contact has multiple', () => {
    const ctx = createTestContext()
    ctx.runOK(
      'contact', 'add', '--name', 'Jane Doe',
      '--email', 'jane@acme.com', '--email', 'jane.doe@gmail.com',
    )

    const show1 = ctx.runOK('contact', 'show', 'jane@acme.com')
    const show2 = ctx.runOK('contact', 'show', 'jane.doe@gmail.com')
    expect(show1).toContain('Jane Doe')
    expect(show2).toContain('Jane Doe')
  })

  test('duplicate check applies across all emails', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'Jane', '--email', 'jane@acme.com', '--email', 'jane@personal.com')
    // Adding a new contact with jane@personal.com should fail — it belongs to Jane.
    const result = ctx.runFail('contact', 'add', '--name', 'Other Jane', '--email', 'jane@personal.com')
    expect(result.stderr).toContain('duplicate')
  })
})

describe('contact show', () => {
  test('by email', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'Jane Doe', '--email', 'jane@acme.com')
    const out = ctx.runOK('contact', 'show', 'jane@acme.com')
    expect(out).toContain('Jane Doe')
  })

  test('not found returns error', () => {
    const ctx = createTestContext()
    ctx.runFail('contact', 'show', 'nonexistent@example.com')
  })
})

describe('contact list', () => {
  test('empty database returns empty array', () => {
    const ctx = createTestContext()
    const contacts = ctx.runJSON<unknown[]>('contact', 'list', '--format', 'json')
    expect(contacts).toEqual([])
  })

  test('returns all contacts', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'Alice', '--email', 'alice@example.com')
    ctx.runOK('contact', 'add', '--name', 'Bob', '--email', 'bob@example.com')
    ctx.runOK('contact', 'add', '--name', 'Charlie', '--email', 'charlie@example.com')

    const contacts = ctx.runJSON<unknown[]>('contact', 'list', '--format', 'json')
    expect(contacts).toHaveLength(3)
  })

  test('filter by tag', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'Alice', '--email', 'alice@example.com', '--tag', 'vip')
    ctx.runOK('contact', 'add', '--name', 'Bob', '--email', 'bob@example.com')

    const contacts = ctx.runJSON<unknown[]>('contact', 'list', '--tag', 'vip', '--format', 'json')
    expect(contacts).toHaveLength(1)
  })

  test('filter by company', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'Alice', '--email', 'alice@acme.com', '--company', 'Acme')
    ctx.runOK('contact', 'add', '--name', 'Bob', '--email', 'bob@other.com', '--company', 'Other')

    const contacts = ctx.runJSON<unknown[]>('contact', 'list', '--company', 'Acme', '--format', 'json')
    expect(contacts).toHaveLength(1)
  })

  test('sort by name', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'Charlie')
    ctx.runOK('contact', 'add', '--name', 'Alice')
    ctx.runOK('contact', 'add', '--name', 'Bob')

    const contacts = ctx.runJSON<Array<{ name: string }>>('contact', 'list', '--sort', 'name', '--format', 'json')
    expect(contacts.map((c) => c.name)).toEqual(['Alice', 'Bob', 'Charlie'])
  })

  test('limit and offset', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'A')
    ctx.runOK('contact', 'add', '--name', 'B')
    ctx.runOK('contact', 'add', '--name', 'C')
    ctx.runOK('contact', 'add', '--name', 'D')

    const page1 = ctx.runJSON<unknown[]>('contact', 'list', '--limit', '2', '--format', 'json')
    expect(page1).toHaveLength(2)

    const page2 = ctx.runJSON<unknown[]>('contact', 'list', '--limit', '2', '--offset', '2', '--format', 'json')
    expect(page2).toHaveLength(2)
  })

  test('format ids outputs one ID per line', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'Alice')
    ctx.runOK('contact', 'add', '--name', 'Bob')

    const out = ctx.runOK('contact', 'list', '--format', 'ids')
    const lines = out.trim().split('\n')
    expect(lines).toHaveLength(2)
    for (const line of lines) {
      expect(line).toStartWith('ct_')
    }
  })

  test('format csv has header and data rows', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'Alice', '--email', 'alice@example.com')

    const out = ctx.runOK('contact', 'list', '--format', 'csv')
    const lines = out.trim().split('\n')
    expect(lines.length).toBeGreaterThanOrEqual(2)
    expect(lines[0]).toContain('name')
    expect(lines[0]).toContain('email')
    expect(lines[1]).toContain('Alice')
    expect(lines[1]).toContain('alice@example.com')
  })

  test('filter expression', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'Alice', '--title', 'CTO', '--source', 'conference')
    ctx.runOK('contact', 'add', '--name', 'Bob', '--title', 'Engineer', '--source', 'inbound')
    ctx.runOK('contact', 'add', '--name', 'Charlie', '--title', 'CTO', '--source', 'inbound')

    const contacts = ctx.runJSON<unknown[]>(
      'contact', 'list', '--filter', 'title=CTO AND source=inbound', '--format', 'json',
    )
    expect(contacts).toHaveLength(1)
  })
})

describe('contact edit', () => {
  test('update fields by ID', () => {
    const ctx = createTestContext()
    const id = ctx.runOK('contact', 'add', '--name', 'Jane Doe', '--email', 'jane@acme.com', '--title', 'Engineer').trim()
    ctx.runOK('contact', 'edit', id, '--name', 'Jane Smith', '--title', 'CTO')

    const show = ctx.runOK('contact', 'show', id)
    expect(show).toContain('Jane Smith')
    expect(show).toContain('CTO')
    expect(show).not.toContain('Jane Doe')
  })

  test('update by email', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'Jane Doe', '--email', 'jane@acme.com')
    ctx.runOK('contact', 'edit', 'jane@acme.com', '--title', 'CEO')

    const show = ctx.runOK('contact', 'show', 'jane@acme.com')
    expect(show).toContain('CEO')
  })

  test('set and unset custom fields', () => {
    const ctx = createTestContext()
    const id = ctx.runOK('contact', 'add', '--name', 'Jane', '--set', 'github=janedoe').trim()

    ctx.runOK('contact', 'edit', id, '--set', 'github=janesmith')
    expect(ctx.runOK('contact', 'show', id)).toContain('janesmith')

    ctx.runOK('contact', 'edit', id, '--unset', 'github')
    expect(ctx.runOK('contact', 'show', id)).not.toContain('github')
  })

  test('add and remove tags', () => {
    const ctx = createTestContext()
    const id = ctx.runOK('contact', 'add', '--name', 'Jane', '--tag', 'lead').trim()
    ctx.runOK('contact', 'edit', id, '--add-tag', 'vip', '--rm-tag', 'lead')

    const show = ctx.runOK('contact', 'show', id)
    expect(show).toContain('vip')
    expect(show).not.toContain('lead')
  })

  test('add email to existing contact', () => {
    const ctx = createTestContext()
    const id = ctx.runOK('contact', 'add', '--name', 'Jane', '--email', 'jane@acme.com').trim()
    ctx.runOK('contact', 'edit', id, '--add-email', 'jane.doe@gmail.com')

    const show = ctx.runOK('contact', 'show', id)
    expect(show).toContain('jane@acme.com')
    expect(show).toContain('jane.doe@gmail.com')
  })

  test('remove email from contact', () => {
    const ctx = createTestContext()
    const id = ctx.runOK('contact', 'add', '--name', 'Jane', '--email', 'jane@acme.com', '--email', 'old@acme.com').trim()
    ctx.runOK('contact', 'edit', id, '--rm-email', 'old@acme.com')

    const show = ctx.runOK('contact', 'show', id)
    expect(show).toContain('jane@acme.com')
    expect(show).not.toContain('old@acme.com')
  })

  test('add phone to existing contact', () => {
    const ctx = createTestContext()
    const id = ctx.runOK('contact', 'add', '--name', 'Jane', '--phone', '+1-555-0100').trim()
    ctx.runOK('contact', 'edit', id, '--add-phone', '+44-20-7946-0958')

    const show = ctx.runOK('contact', 'show', id)
    expect(show).toContain('+1-555-0100')
    expect(show).toContain('+44-20-7946-0958')
  })

  test('remove phone from contact', () => {
    const ctx = createTestContext()
    const id = ctx.runOK('contact', 'add', '--name', 'Jane', '--phone', '+1-555-0100', '--phone', '+1-555-OLD').trim()
    ctx.runOK('contact', 'edit', id, '--rm-phone', '+1-555-OLD')

    const show = ctx.runOK('contact', 'show', id)
    expect(show).toContain('+1-555-0100')
    expect(show).not.toContain('+1-555-OLD')
  })
})

describe('contact rm', () => {
  test('delete by ID', () => {
    const ctx = createTestContext()
    const id = ctx.runOK('contact', 'add', '--name', 'Jane', '--email', 'jane@acme.com').trim()
    ctx.runOK('contact', 'rm', id, '--force')
    ctx.runFail('contact', 'show', id)
  })

  test('delete by email', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'Jane', '--email', 'jane@acme.com')
    ctx.runOK('contact', 'rm', 'jane@acme.com', '--force')
    ctx.runFail('contact', 'show', 'jane@acme.com')
  })
})

describe('contact merge', () => {
  test('merges two contacts keeping first', () => {
    const ctx = createTestContext()
    const id1 = ctx.runOK('contact', 'add', '--name', 'Jane Doe', '--email', 'jane@acme.com', '--tag', 'vip').trim()
    const id2 = ctx.runOK('contact', 'add', '--name', 'J. Doe', '--email', 'jane.doe@gmail.com', '--tag', 'enterprise').trim()

    ctx.runOK('contact', 'merge', id1, id2, '--keep-first')

    const show = ctx.runOK('contact', 'show', id1)
    expect(show).toContain('jane@acme.com')
    expect(show).toContain('jane.doe@gmail.com')
    expect(show).toContain('vip')
    expect(show).toContain('enterprise')

    ctx.runFail('contact', 'show', id2)
  })
})
