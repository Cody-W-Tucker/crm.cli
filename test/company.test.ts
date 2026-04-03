import { describe, expect, test } from 'bun:test'

import { createTestContext } from './helpers.ts'

describe('company add', () => {
  test('basic add returns prefixed ID', () => {
    const ctx = createTestContext()
    const out = ctx.runOK('company', 'add', '--name', 'Acme Corp')
    expect(out.trim()).toStartWith('co_')
  })

  test('full add stores all fields', () => {
    const ctx = createTestContext()
    const id = ctx
      .runOK(
        'company', 'add',
        '--name', 'Acme Corp',
        '--domain', 'acme.com',
        '--industry', 'SaaS',
        '--size', '50-200',
        '--tag', 'enterprise',
        '--set', 'founded=2020',
      )
      .trim()

    const show = ctx.runOK('company', 'show', id)
    expect(show).toContain('Acme Corp')
    expect(show).toContain('acme.com')
    expect(show).toContain('SaaS')
    expect(show).toContain('50-200')
    expect(show).toContain('enterprise')
    expect(show).toContain('2020')
  })

  test('fails without --name', () => {
    const ctx = createTestContext()
    const result = ctx.runFail('company', 'add', '--domain', 'acme.com')
    expect(result.stderr).toContain('name')
  })
})

describe('company show', () => {
  test('by domain', () => {
    const ctx = createTestContext()
    ctx.runOK('company', 'add', '--name', 'Acme Corp', '--domain', 'acme.com')
    const out = ctx.runOK('company', 'show', 'acme.com')
    expect(out).toContain('Acme Corp')
  })

  test('shows linked contacts', () => {
    const ctx = createTestContext()
    ctx.runOK('company', 'add', '--name', 'Acme Corp', '--domain', 'acme.com')
    ctx.runOK('contact', 'add', '--name', 'Jane Doe', '--email', 'jane@acme.com', '--company', 'Acme Corp')
    ctx.runOK('contact', 'add', '--name', 'John Doe', '--email', 'john@acme.com', '--company', 'Acme Corp')

    const show = ctx.runOK('company', 'show', 'acme.com')
    expect(show).toContain('Jane Doe')
    expect(show).toContain('John Doe')
  })
})

describe('company list', () => {
  test('returns all companies', () => {
    const ctx = createTestContext()
    ctx.runOK('company', 'add', '--name', 'Acme Corp', '--industry', 'SaaS')
    ctx.runOK('company', 'add', '--name', 'Globex', '--industry', 'Manufacturing')
    ctx.runOK('company', 'add', '--name', 'Initech', '--industry', 'SaaS')

    const companies = ctx.runJSON<unknown[]>('company', 'list', '--format', 'json')
    expect(companies).toHaveLength(3)
  })

  test('filter by tag', () => {
    const ctx = createTestContext()
    ctx.runOK('company', 'add', '--name', 'Acme Corp', '--tag', 'enterprise')
    ctx.runOK('company', 'add', '--name', 'Small Co')

    const companies = ctx.runJSON<unknown[]>('company', 'list', '--tag', 'enterprise', '--format', 'json')
    expect(companies).toHaveLength(1)
  })
})

describe('company edit', () => {
  test('update fields', () => {
    const ctx = createTestContext()
    const id = ctx.runOK('company', 'add', '--name', 'Acme Corp').trim()
    ctx.runOK('company', 'edit', id, '--name', 'Acme Inc', '--industry', 'Tech')

    const show = ctx.runOK('company', 'show', id)
    expect(show).toContain('Acme Inc')
    expect(show).toContain('Tech')
  })

  test('edit by domain', () => {
    const ctx = createTestContext()
    ctx.runOK('company', 'add', '--name', 'Acme Corp', '--domain', 'acme.com')
    ctx.runOK('company', 'edit', 'acme.com', '--industry', 'Fintech')

    const show = ctx.runOK('company', 'show', 'acme.com')
    expect(show).toContain('Fintech')
  })
})

describe('company rm', () => {
  test('delete company', () => {
    const ctx = createTestContext()
    const id = ctx.runOK('company', 'add', '--name', 'Acme Corp').trim()
    ctx.runOK('company', 'rm', id, '--force')
    ctx.runFail('company', 'show', id)
  })

  test('does not delete linked contacts', () => {
    const ctx = createTestContext()
    const coID = ctx.runOK('company', 'add', '--name', 'Acme Corp').trim()
    const ctID = ctx.runOK('contact', 'add', '--name', 'Jane', '--email', 'jane@acme.com', '--company', 'Acme Corp').trim()

    ctx.runOK('company', 'rm', coID, '--force')
    const show = ctx.runOK('contact', 'show', ctID)
    expect(show).toContain('Jane')
  })
})

describe('company auto-creation', () => {
  test('contact add with --company auto-creates company stub', () => {
    const ctx = createTestContext()
    ctx.runOK('contact', 'add', '--name', 'Jane', '--company', 'NewCo')

    const companies = ctx.runJSON<unknown[]>('company', 'list', '--format', 'json')
    expect(companies).toHaveLength(1)
  })
})
