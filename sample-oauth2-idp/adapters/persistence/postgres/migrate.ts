/**
 * Layer 4 — Adapter Layer (migration runner)
 *
 * infra/migrations/*.sql を順次適用する。
 * schema_migrations テーブルを使って適用済みを記録し、未適用のみ実行する。
 *
 * 使い方:
 *   DATABASE_URL=postgres://... bun run migrate:up
 *   DATABASE_URL=postgres://... bun run migrate:status
 */

import { createHash } from 'crypto'
import { readdir, readFile } from 'fs/promises'
import { join } from 'path'
import { getPool } from './pool'

interface MigrationFile {
  version: string
  filename: string
  sql: string
  checksum: string
}

async function loadMigrations(specDir: string): Promise<MigrationFile[]> {
  const entries = await readdir(specDir)
  const sqls = entries.filter((f) => /^\d{4}_.*\.sql$/.test(f)).sort()
  const results: MigrationFile[] = []
  for (const filename of sqls) {
    const sql = await readFile(join(specDir, filename), 'utf-8')
    const version = filename.split('_')[0]
    const checksum = createHash('sha256').update(sql).digest('hex')
    results.push({ version, filename, sql, checksum })
  }
  return results
}

async function ensureMigrationsTable(pool: any): Promise<void> {
  await pool.query(`
    CREATE TABLE IF NOT EXISTS schema_migrations (
      version    TEXT PRIMARY KEY,
      applied_at TIMESTAMPTZ NOT NULL DEFAULT now(),
      checksum   TEXT NOT NULL
    )
  `)
}

export async function migrateUp(specDir: string, databaseUrl: string): Promise<void> {
  const pool = await getPool({ connectionString: databaseUrl })
  await ensureMigrationsTable(pool)

  const migrations = await loadMigrations(specDir)
  const { rows } = await pool.query(`SELECT version, checksum FROM schema_migrations`)
  const applied = new Map<string, string>(rows.map((r: any) => [r.version, r.checksum]))

  for (const m of migrations) {
    const prior = applied.get(m.version)
    if (prior === undefined) {
      // eslint-disable-next-line no-console
      console.log(`[migrate] applying ${m.filename}`)
      const client = await pool.connect()
      try {
        await client.query('BEGIN')
        await client.query(m.sql)
        await client.query(`INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)`, [
          m.version,
          m.checksum,
        ])
        await client.query('COMMIT')
      } catch (err) {
        await client.query('ROLLBACK')
        throw err
      } finally {
        client.release()
      }
    } else if (prior !== m.checksum) {
      throw new Error(
        `[migrate] checksum mismatch for ${m.filename}: ` +
          `applied=${prior}, current=${m.checksum}. ` +
          `Migrations are immutable once applied (infra/migrations/README.md).`,
      )
    } else {
      // eslint-disable-next-line no-console
      console.log(`[migrate] skip ${m.filename} (already applied)`)
    }
  }
}

export async function migrateStatus(specDir: string, databaseUrl: string): Promise<void> {
  const pool = await getPool({ connectionString: databaseUrl })
  await ensureMigrationsTable(pool)
  const migrations = await loadMigrations(specDir)
  const { rows } = await pool.query(
    `SELECT version, applied_at FROM schema_migrations ORDER BY version`,
  )
  const applied = new Map<string, Date>(rows.map((r: any) => [r.version, r.applied_at]))
  for (const m of migrations) {
    const ts = applied.get(m.version)
    // eslint-disable-next-line no-console
    console.log(`${m.version}  ${ts ? `applied at ${ts.toISOString()}` : 'PENDING'}  ${m.filename}`)
  }
}

// CLI entry point
if (import.meta.main) {
  const cmd = process.argv[2] ?? 'up'
  const databaseUrl = process.env.DATABASE_URL
  if (!databaseUrl) {
    console.error('DATABASE_URL is required')
    process.exit(1)
  }
  const specDir =
    process.env.MIGRATIONS_DIR ?? new URL('../../../infra/migrations', import.meta.url).pathname

  const main = async () => {
    if (cmd === 'up') {
      await migrateUp(specDir, databaseUrl)
    } else if (cmd === 'status') {
      await migrateStatus(specDir, databaseUrl)
    } else {
      console.error(`unknown command: ${cmd}`)
      process.exit(1)
    }
  }
  main()
    .then(() => process.exit(0))
    .catch((e) => {
      console.error(e)
      process.exit(1)
    })
}
