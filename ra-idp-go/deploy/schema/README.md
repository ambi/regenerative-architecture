# PostgreSQL schema workflow

`postgres.sql` is the declarative current-state schema for the PostgreSQL
adapter. It is applied with `psqldef`, the PostgreSQL command in the sqldef tool
family.

## Install psqldef

On macOS:

```bash
brew install sqldef/sqldef/psqldef
```

On Linux, download the pre-built `psqldef` binary from the sqldef releases page,
or use the sqldef Docker image in the deploy job. Pin the version in CI/CD jobs
instead of using an unqualified latest binary.

Confirm the installed command:

```bash
psqldef --version
```

## Connection variables

For local dev compose, the equivalent connection is:

```bash
export PGHOST=localhost
export PGPORT=5432
export PGUSER=ra_idp
export PGPASSWORD=ra_idp
export PGDATABASE=ra_idp
```

`psqldef` uses `psql`-style connection options, so production deploy jobs should
map their `DATABASE_URL` secret to `PGHOST` / `PGPORT` / `PGUSER` /
`PGPASSWORD` / `PGDATABASE` before running the workflow.

## Change workflow

1. Edit `deploy/schema/postgres.sql` to the desired current schema.
2. If the change needs data movement, add an explicit runbook or purpose-built
   SQL script for the backfill / value conversion. Do not hide data movement in
   the declarative schema file.
3. Generate the planned DDL without applying it:

```bash
psqldef -U "$PGUSER" -h "$PGHOST" -p "$PGPORT" "$PGDATABASE" \
  --dry-run < deploy/schema/postgres.sql \
  | tee /tmp/ra-idp-go-schema-plan.sql
```

4. Review `/tmp/ra-idp-go-schema-plan.sql`.
   - Empty output means the database already matches the current schema.
   - `DROP` operations require explicit human review and must not be enabled in
     automation by default.
   - Long-locking operations, type changes, and NOT NULL additions on populated
     tables need a separate rollout plan.
5. Apply the reviewed schema change:

```bash
psqldef -U "$PGUSER" -h "$PGHOST" -p "$PGPORT" "$PGDATABASE" \
  --apply < deploy/schema/postgres.sql
```

6. Run dry-run again. The expected result is empty output:

```bash
psqldef -U "$PGUSER" -h "$PGHOST" -p "$PGPORT" "$PGDATABASE" \
  --dry-run < deploy/schema/postgres.sql
```

7. Record the generated plan and the final empty dry-run in the WI completion or
   release evidence.

## Local Docker development

The dev compose file has a one-shot `schema` service:

```bash
docker compose -f deploy/docker/docker-compose.dev.yaml up --build
```

`schema` waits for PostgreSQL, runs `psqldef --apply --file
/schema/postgres.sql`, exits, and then `idp` starts. The apply step is
idempotent; running compose again should not produce additional DDL after the
database matches `postgres.sql`.

When only the schema changed and the stack is already running, apply it without
recreating the whole stack:

```bash
docker compose -f deploy/docker/docker-compose.dev.yaml run --rm schema
```

To inspect the dev database before applying, run dry-run from the host after
installing `psqldef`:

```bash
psqldef -U ra_idp -h localhost -p 5432 ra_idp \
  --dry-run < deploy/schema/postgres.sql
```

## Production deployment

The application does not apply schema changes at startup. Production uses an
explicit deploy step before starting the new application version.

First deployment to an empty database:

```bash
psqldef -U "$PGUSER" -h "$PGHOST" -p "$PGPORT" "$PGDATABASE" \
  --dry-run < deploy/schema/postgres.sql
psqldef -U "$PGUSER" -h "$PGHOST" -p "$PGPORT" "$PGDATABASE" \
  --apply < deploy/schema/postgres.sql
psqldef -U "$PGUSER" -h "$PGHOST" -p "$PGPORT" "$PGDATABASE" \
  --check < deploy/schema/postgres.sql
```

Second and later deployments use the same sequence. `--dry-run` shows the DDL
needed to move the existing database to the new desired schema. After review,
`--apply` performs it, and `--check` must return no pending DDL before the new
application version is promoted.

If the dry-run contains destructive or long-locking changes, stop and create a
separate rollout plan. Do not add `--enable-drop` to automated production jobs
without explicit approval for that release.

## Empty database bootstrap

For a new PostgreSQL database, apply `postgres.sql` directly with the same
`--apply` command. Reference data is not part of this file; the application
converges required rows such as the default tenant at startup.

## Rules

- Keep structural schema in `postgres.sql`.
- Keep data migrations, backfills, and high-risk destructive changes as explicit
  SQL scripts or runbooks outside `deploy/schema/`.
- Do not put reference data in this file. The application converges required
  reference data such as the default tenant at startup.
- Do not reintroduce an application startup migration runner. Schema changes are
  a deploy-time operation.
- Do not use `--enable-drop` in automation without a reviewed migration plan.
