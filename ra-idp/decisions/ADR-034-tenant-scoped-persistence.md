# ADR-034: テナント単位の永続化境界とクロステナント分離

## ステータス

採用

## コンテキスト

ADR-032 でテナント集約を導入し、ADR-033 で HTTP 境界での解決方式を
決めた。本 ADR は永続化層での分離戦略を定める:

- どのテーブル / ストアが `tenant_id` カラムを explicit に持つか
- どのテーブルが親集約 (User / Client) 経由で transitive に scope されるか
- PK / unique 制約の再構成方針
- Redis ストアの key namespace 方針
- cross-tenant ルックアップが起きないことをコードレベルでどう保証するか

belt-and-suspenders に全テーブルへ `tenant_id` を加える案も検討したが、
非機能影響 (index 肥大化 / FK 多重化 / マイグレーション複雑度) と
ロジック上の冗長性のバランスから、aggregate root に explicit、child に
transitive、という方針が最も保守可能と判断した。

## 決定

### 1. Explicit `tenant_id TEXT NOT NULL` を持つテーブル

| テーブル | 理由 |
|---|---|
| `clients` | aggregate root。`client_id` は per-tenant 一意 |
| `users` | aggregate root。`preferred_username` は per-tenant 一意、`sub` は global 一意 |
| `consents` | (sub, client_id) は両者が tenant 内で参照されるため、PK に tenant_id を加える |
| `refresh_tokens` | `/token` のリプレイ検出が join なしで効くために必要 |
| `authorization_codes` | 同上、`/token` リプレイ・コード redeem の cross-tenant 拒否を安価に効かせる |
| `par_records` | `/authorize` の引き継ぎで cross-tenant request_uri を即時拒否するため |
| `device_authorizations` | device flow の cross-tenant 拒否を安価に効かせるため |
| `authorization_requests` | `/authorize` セッション内ステートを per-tenant に閉じるため |

### 2. Transitive に scope されるテーブル / ストア (カラム追加なし)

| テーブル / ストア | 親 | 理由 |
|---|---|---|
| `password_history` | `users(sub)` | `sub` が global 一意かつ user に紐付くため重複情報になる |
| `password_reset_tokens` | `users(sub)` | 同上。トークン consume は `sub` 経由 |
| `mfa_factors` | `users(sub)` | 同上 |
| login throttle keys | `users(preferred_username)` 等 | per-account 単位は (tenant_id, username) で識別するが、ストア層では key 文字列に `tenant_id` を埋め込む。テーブル追加なし |
| sessions | `users(sub)` | レコードにも `tenant_id` を保持して HTTP 境界で即時照合するが、永続 DB の追加テーブルは不要。ADR-031 の session 失効ガードと同じ経路 |
| DPoP replay cache / access token denylist / client assertion replay | `jti` / token hash | 値が global 一意なので衝突なし。Redis namespace は per-tenant に分ける (項 5) |

### 3. PK / unique / FK 再構成

```sql
-- clients
PRIMARY KEY (tenant_id, client_id)
FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT

-- users
UNIQUE INDEX (tenant_id, preferred_username) WHERE deleted_at IS NULL
-- sub の global PK は維持
FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE RESTRICT

-- consents
PRIMARY KEY (tenant_id, sub, client_id)
FOREIGN KEY (tenant_id, client_id) REFERENCES clients(tenant_id, client_id)
FOREIGN KEY (tenant_id) REFERENCES tenants(id)

-- refresh_tokens
PRIMARY KEY (id)  -- UUID、global 一意のまま
INDEX (tenant_id, sub)
INDEX (tenant_id, client_id)
FOREIGN KEY (tenant_id, client_id) REFERENCES clients(tenant_id, client_id)
FOREIGN KEY (tenant_id) REFERENCES tenants(id)
```

### 4. クロステナントルックアップの即時拒否

tenant-scoped aggregate の repository メソッドは `tenant_id` を
**第一引数**で受け取る:

```ts
findById(tenant_id: string, client_id: string): Promise<Client | null>
findByUsername(tenant_id: string, username: string): Promise<User | null>
```

`findBySub(sub)` は `sub` が global 一意のため signature は単一引数のまま
維持するが、解決後の `user.tenant_id !== ctx.tenant_id` を上位で検証する。
これは session resolution や `/userinfo` 等で必要。

token endpoint 系 (refresh / device / authorization_code 交換):

```ts
const record = await refreshStore.findByHash(hash)
if (!record || record.tenant_id !== ctx.tenant_id) {
  throw new OAuthError('invalid_grant', '...')
}
```

応答メッセージは存在しない場合と一致させ、tenant 存在を漏らさない。

### 5. Redis key namespace

すべての Redis ストアで key の先頭に `tenant:{id}:` を付与する。
影響を受けるストア:

- 認可リクエスト (`auth_request:`)
- 認可コード (`auth_code:`)
- PAR (`par:`)
- device code (`device:`)
- session (`session:`)
- DPoP replay (`dpop_replay:`)
- access token denylist (`token_denylist:`)
- login throttle (`throttle:account:` / `throttle:ip:`)
- client assertion replay (`client_assertion:`)

→ `tenant:{id}:auth_request:{...}` 等。

**In-flight state のマイグレーションは行わない**。TTL は最長で 24 時間
程度なので、デプロイ直後の数時間は古い key が cache に残っても自然消滅
する。README にこのことを明記する。

### 6. `sub` を global 一意に維持する

`User.sub` は OIDC subject identifier として RP 側で永続的に保管される
値。tenant ID を埋め込んだ複合 sub にすると:

- RP 側 DB に sub 列を 2 倍に拡張する必要が出る
- ID Token の他フィールド (`aud`, `iss`) と整合させる際の心智的負荷が増える
- (将来) pairwise subject identifier 採用時のロジックが二重化する

global 一意を維持しつつ、`sub` から resolve した `user.tenant_id` を
ガード値に使う。`UserRepository.findBySub(sub)` は単一引数のまま。

## 影響

- migration `0007_tenants.sql` で `tenants` テーブルを追加、aggregate root
  テーブルに `tenant_id` 列を追加 (default `'default'`)、PR-A はここまで。
- migration `0008_tenant_pk_recompose.sql` で PK / unique / FK 再構成、
  Redis key prefix 切替、cross-tenant guard を入れる (PR-B)。
- すべてのテストフィクスチャに `tenant_id: 'default'` を明示する必要が
  あるが、Zod schema のデフォルトで暗黙に乗せるため変更は最小化される。
- monitoring / metrics の cardinality は tenant ごとに増えるため、
  Phase 8 では Prometheus labels の `tenant_id` 採用を別 ADR で議論する。
- 監査イベントの payload には既存通り `client_id` / `sub` を載せるが、
  PR-B 以降は `tenant_id` を明示するイベントスキーマ拡張を検討する
  (現フェーズではイベント payload は無変更で進む)。

## 却下した代替案

- **全テーブルに `tenant_id` を追加する** — index 肥大化と FK の冗長性。
  child 側で `tenant_id` を持っても親集約と必ず一致するため、整合性は
  逆に application 層で再チェックする必要が出る。
- **`sub` に tenant_id を埋め込む (`tenant:user-uuid`)** — RP 側互換性が
  破壊的に変わる。pairwise subject identifier との整合も難しくなる。
- **Postgres Row-Level Security (RLS) を採用する** — 強力だが本 repo の
  接続プーリング (`pg.Pool`) を per-tenant に分割する必要があり、
  Bun runtime での実用性が乏しい。アプリ層での明示的ガードを採る。
- **Redis を tenant 別 instance に分割** — 運用負荷が高い。namespace prefix で
  論理分離する方が Phase 4 のスコープに合う。物理分離は Phase 8 の
  検討事項。
