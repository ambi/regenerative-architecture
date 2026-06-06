-- Migration 0002 — Enforce single active signing key (ADR-009)
-- Layer 1: Specification Core (data model authority)
--
-- signing_keys は「同時に active = TRUE はちょうど 1 つ」という不変条件を持つ (ADR-009)。
-- 0001 の signing_keys_active_idx は検索用で一意性を強制しないため、ここで
-- 部分一意インデックスを追加し、複数レプリカ同時起動 / 同時 rotate 時の
-- 二重 active を DB レベルで防ぐ。
--
-- これにより PostgresKeyStore は:
--   - 起動時シード: INSERT ... ON CONFLICT DO NOTHING で「最初の 1 つだけ」が active になる
--   - rotate(): トランザクション内で旧 active を倒してから新 active を挿入できる
-- を競合安全に行える。
--
-- migrate.ts が各マイグレーションをトランザクションで包むため、本ファイルは BEGIN/COMMIT を含めない
-- (加法的変更原則: infra/migrations/README.md)。

CREATE UNIQUE INDEX IF NOT EXISTS signing_keys_single_active_idx
    ON signing_keys (active)
    WHERE active;
