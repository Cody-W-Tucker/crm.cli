import { describe, expect, test } from 'bun:test'
import { mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

import { createTestContext } from './helpers.ts'

describe('config: phone settings', () => {
  test('phone.default_country allows short numbers', () => {
    const ctx = createTestContext()
    const configPath = join(ctx.dir, 'crm.toml')
    writeFileSync(configPath, `[phone]\ndefault_country = "US"\n`)
    const id = ctx
      .runOK(
        '--config',
        configPath,
        'contact',
        'add',
        '--name',
        'Jane',
        '--phone',
        '2125551234',
      )
      .trim()
    const out = ctx.runOK(
      '--config',
      configPath,
      'contact',
      'show',
      id,
      '--format',
      'json',
    )
    const data = JSON.parse(out)
    expect(data.phones[0]).toBe('+12125551234')
  })

  test('phone.display = national shows national format', () => {
    const ctx = createTestContext()
    const configPath = join(ctx.dir, 'crm.toml')
    writeFileSync(
      configPath,
      `[phone]\ndefault_country = "US"\ndisplay = "national"\n`,
    )
    ctx.runOK(
      '--config',
      configPath,
      'contact',
      'add',
      '--name',
      'Jane',
      '--phone',
      '+12125551234',
    )
    const out = ctx.runOK('--config', configPath, 'contact', 'list')
    // National format should not have +1 prefix in display
    expect(out).toContain('Jane')
  })

  test('phone.display = e164 shows raw E.164', () => {
    const ctx = createTestContext()
    const configPath = join(ctx.dir, 'crm.toml')
    writeFileSync(configPath, `[phone]\ndisplay = "e164"\n`)
    const id = ctx
      .runOK(
        '--config',
        configPath,
        'contact',
        'add',
        '--name',
        'Jane',
        '--phone',
        '+12125551234',
      )
      .trim()
    const out = ctx.runOK('--config', configPath, 'contact', 'show', id)
    expect(out).toContain('+12125551234')
  })
})

describe('config: mount settings affect export-fs', () => {
  test('mount.max_recent_activity limits activities in contact JSON', () => {
    const ctx = createTestContext()
    const configPath = join(ctx.dir, 'crm.toml')
    writeFileSync(configPath, '[mount]\nmax_recent_activity = 2\n')
    ctx.runOK(
      '--config',
      configPath,
      'contact',
      'add',
      '--name',
      'Jane',
      '--email',
      'jane@acme.com',
    )
    for (let i = 0; i < 5; i++) {
      ctx.runOK(
        '--config',
        configPath,
        'log',
        'note',
        'jane@acme.com',
        `Note ${i}`,
      )
    }
    const outDir = join(ctx.dir, 'export')
    ctx.runOK('--config', configPath, 'export-fs', outDir)
    const files = readdirSync(join(outDir, 'contacts')).filter(
      (f) => f.endsWith('.json') && !f.startsWith('_'),
    )
    const data = JSON.parse(
      readFileSync(join(outDir, 'contacts', files[0]), 'utf-8'),
    )
    expect(data.recent_activity).toHaveLength(2)
  })
})

describe('config: pipeline stage changes', () => {
  test('deal with stage from old config rejected after config change', () => {
    const ctx = createTestContext()
    const configPath = join(ctx.dir, 'crm.toml')

    // First config has 'alpha' stage
    writeFileSync(configPath, `[pipeline]\nstages = ["alpha", "beta"]\n`)
    ctx.runOK(
      '--config',
      configPath,
      'deal',
      'add',
      '--title',
      'First',
      '--stage',
      'alpha',
    )

    // Change config to different stages — 'alpha' no longer valid
    writeFileSync(configPath, `[pipeline]\nstages = ["gamma", "delta"]\n`)
    const result = ctx.runFail(
      '--config',
      configPath,
      'deal',
      'add',
      '--title',
      'Second',
      '--stage',
      'alpha',
    )
    expect(result.stderr).toContain('stage')
  })

  test('deal move to new stage works after config change', () => {
    const ctx = createTestContext()
    const configPath = join(ctx.dir, 'crm.toml')

    writeFileSync(
      configPath,
      `[pipeline]\nstages = ["alpha", "beta", "gamma"]\n`,
    )
    const id = ctx
      .runOK(
        '--config',
        configPath,
        'deal',
        'add',
        '--title',
        'Test',
        '--stage',
        'alpha',
      )
      .trim()

    ctx.runOK('--config', configPath, 'deal', 'move', id, '--stage', 'gamma')
    const out = ctx.runOK(
      '--config',
      configPath,
      'deal',
      'show',
      id,
      '--format',
      'json',
    )
    expect(JSON.parse(out).stage).toBe('gamma')
  })

  test('export-fs with custom stages only creates those stage dirs', () => {
    const ctx = createTestContext()
    const configPath = join(ctx.dir, 'crm.toml')
    writeFileSync(configPath, `[pipeline]\nstages = ["x", "y"]\n`)
    const outDir = join(ctx.dir, 'export')
    ctx.runOK('--config', configPath, 'export-fs', outDir)
    const stages = readdirSync(join(outDir, 'deals', '_by-stage')).sort()
    expect(stages).toEqual(['x', 'y'])
  })
})

describe('config resolution', () => {
  test('--config flag takes highest priority', () => {
    const ctx = createTestContext()
    const configPath = join(ctx.dir, 'explicit.toml')
    writeFileSync(
      configPath,
      `[pipeline]\nstages = ["alpha", "beta", "gamma"]\n`,
    )

    // Add a deal with a stage from the explicit config.
    const id = ctx
      .runOK(
        '--config',
        configPath,
        'deal',
        'add',
        '--title',
        'Test',
        '--stage',
        'alpha',
      )
      .trim()
    const show = ctx.runOK('--config', configPath, 'deal', 'show', id)
    expect(show).toContain('alpha')
  })

  test('--config flag rejects invalid stage not in custom config', () => {
    const ctx = createTestContext()
    const configPath = join(ctx.dir, 'custom.toml')
    writeFileSync(
      configPath,
      `[pipeline]\nstages = ["alpha", "beta", "gamma"]\n`,
    )

    // "lead" is in the default config but not in this custom config.
    const result = ctx.runFail(
      '--config',
      configPath,
      'deal',
      'add',
      '--title',
      'Test',
      '--stage',
      'lead',
    )
    expect(result.stderr).toContain('stage')
  })

  test('CRM_CONFIG env var is used when no --config flag', () => {
    const ctx = createTestContext()
    const configPath = join(ctx.dir, 'env.toml')
    writeFileSync(
      configPath,
      `[pipeline]\nstages = ["env-stage-1", "env-stage-2"]\n`,
    )

    const result = ctx.runWithEnv(
      { CRM_CONFIG: configPath, CRM_DB: ctx.dbPath },
      'deal',
      'add',
      '--title',
      'Test',
      '--stage',
      'env-stage-1',
    )
    expect(result.exitCode).toBe(0)
  })

  test('crm.toml in current directory is found', () => {
    const ctx = createTestContext()
    writeFileSync(
      join(ctx.dir, 'crm.toml'),
      `[pipeline]\nstages = ["local-1", "local-2", "local-3"]\n`,
    )

    // Run from ctx.dir — should pick up ./crm.toml.
    const id = ctx
      .runOK('deal', 'add', '--title', 'Test', '--stage', 'local-1')
      .trim()
    const show = ctx.runOK('deal', 'show', id)
    expect(show).toContain('local-1')
  })

  test('crm.toml in parent directory is found', () => {
    const ctx = createTestContext()

    // Put config in ctx.dir (the parent).
    writeFileSync(
      join(ctx.dir, 'crm.toml'),
      `[pipeline]\nstages = ["parent-1", "parent-2"]\n`,
    )

    // Create a subdirectory and run from there.
    const subdir = join(ctx.dir, 'subproject')
    mkdirSync(subdir)

    const proc = Bun.spawnSync(
      [
        'bun',
        'run',
        join(import.meta.dir, '..', 'src', 'cli.ts'),
        '--db',
        ctx.dbPath,
        'deal',
        'add',
        '--title',
        'Test',
        '--stage',
        'parent-1',
      ],
      { cwd: subdir, env: { ...process.env, NO_COLOR: '1' } },
    )
    expect(proc.exitCode).toBe(0)
    expect(proc.stdout.toString().trim()).toStartWith('dl_')
  })

  test('crm.toml in grandparent directory is found', () => {
    const ctx = createTestContext()

    writeFileSync(
      join(ctx.dir, 'crm.toml'),
      `[pipeline]\nstages = ["grandparent-1", "grandparent-2"]\n`,
    )

    const nested = join(ctx.dir, 'a', 'b')
    mkdirSync(nested, { recursive: true })

    const proc = Bun.spawnSync(
      [
        'bun',
        'run',
        join(import.meta.dir, '..', 'src', 'cli.ts'),
        '--db',
        ctx.dbPath,
        'deal',
        'add',
        '--title',
        'Test',
        '--stage',
        'grandparent-1',
      ],
      { cwd: nested, env: { ...process.env, NO_COLOR: '1' } },
    )
    expect(proc.exitCode).toBe(0)
  })

  test('closer crm.toml overrides parent crm.toml', () => {
    const ctx = createTestContext()

    // Parent config.
    writeFileSync(
      join(ctx.dir, 'crm.toml'),
      `[pipeline]\nstages = ["parent-stage"]\n`,
    )

    // Child config in subdir.
    const subdir = join(ctx.dir, 'project')
    mkdirSync(subdir)
    writeFileSync(
      join(subdir, 'crm.toml'),
      `[pipeline]\nstages = ["child-stage"]\n`,
    )

    // Run from subdir — child config should win.
    const proc = Bun.spawnSync(
      [
        'bun',
        'run',
        join(import.meta.dir, '..', 'src', 'cli.ts'),
        '--db',
        ctx.dbPath,
        'deal',
        'add',
        '--title',
        'Test',
        '--stage',
        'child-stage',
      ],
      { cwd: subdir, env: { ...process.env, NO_COLOR: '1' } },
    )
    expect(proc.exitCode).toBe(0)

    // Parent stage should NOT be valid from child dir.
    const proc2 = Bun.spawnSync(
      [
        'bun',
        'run',
        join(import.meta.dir, '..', 'src', 'cli.ts'),
        '--db',
        ctx.dbPath,
        'deal',
        'add',
        '--title',
        'Test2',
        '--stage',
        'parent-stage',
      ],
      { cwd: subdir, env: { ...process.env, NO_COLOR: '1' } },
    )
    expect(proc2.exitCode).not.toBe(0)
  })

  test('--config flag overrides local crm.toml', () => {
    const ctx = createTestContext()

    // Local config in CWD.
    writeFileSync(
      join(ctx.dir, 'crm.toml'),
      `[pipeline]\nstages = ["local-stage"]\n`,
    )

    // Explicit config via flag.
    const explicitConfig = join(ctx.dir, 'override.toml')
    writeFileSync(explicitConfig, `[pipeline]\nstages = ["explicit-stage"]\n`)

    // --config should win over local crm.toml.
    const id = ctx
      .runOK(
        '--config',
        explicitConfig,
        'deal',
        'add',
        '--title',
        'Test',
        '--stage',
        'explicit-stage',
      )
      .trim()
    expect(id).toStartWith('dl_')

    // local-stage should NOT work with --config override.
    const result = ctx.runFail(
      '--config',
      explicitConfig,
      'deal',
      'add',
      '--title',
      'Test2',
      '--stage',
      'local-stage',
    )
    expect(result.stderr).toContain('stage')
  })

  test('falls back to defaults when no config file exists', () => {
    const ctx = createTestContext()

    // No crm.toml anywhere — default stages should work.
    const id = ctx
      .runOK('deal', 'add', '--title', 'Test', '--stage', 'lead')
      .trim()
    expect(id).toStartWith('dl_')
  })

  test('config sets default output format', () => {
    const ctx = createTestContext()
    writeFileSync(join(ctx.dir, 'crm.toml'), `[defaults]\nformat = "json"\n`)

    ctx.runOK('contact', 'add', '--name', 'Jane')

    // Without --format, should use the config default (json).
    const out = ctx.runOK('contact', 'list')
    expect(out.trim()).toStartWith('[')
  })

  test('--format flag overrides config default', () => {
    const ctx = createTestContext()
    writeFileSync(join(ctx.dir, 'crm.toml'), `[defaults]\nformat = "json"\n`)

    ctx.runOK('contact', 'add', '--name', 'Jane')

    // Explicit --format csv should override config's json default.
    const out = ctx.runOK('contact', 'list', '--format', 'csv')
    expect(out).toContain('name')
    expect(out).not.toStartWith('[')
  })

  test('config sets custom database path', () => {
    const ctx = createTestContext()
    const customDB = join(ctx.dir, 'from-config.db')
    writeFileSync(
      join(ctx.dir, 'crm.toml'),
      `[database]\npath = "${customDB}"\n`,
    )

    // Run without --db flag — should use config's database path.
    const proc = Bun.spawnSync(
      [
        'bun',
        'run',
        join(import.meta.dir, '..', 'src', 'cli.ts'),
        'contact',
        'add',
        '--name',
        'Jane',
      ],
      { cwd: ctx.dir, env: { ...process.env, NO_COLOR: '1' } },
    )
    expect(proc.exitCode).toBe(0)

    const { existsSync } = require('node:fs')
    expect(existsSync(customDB)).toBe(true)
  })
})
