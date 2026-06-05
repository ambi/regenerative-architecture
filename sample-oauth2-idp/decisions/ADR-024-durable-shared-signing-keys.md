# ADR-024: 署名鍵を durable store で複数レプリカ間共有する

## ステータス

採用（ADR-009 を実装に落とす）

## コンテキスト

ADR-009 で「90 日回転・7 日 JWKS オーバーラップ・`SigningKeyRotated` 監査イベント」を定めた。
しかし実装は `InMemoryKeyStore` 固定で、`PERSISTENCE=postgres` でもプロセスローカルだった。
これには本番で致命的な問題がある:

1. **再起動で全鍵が消える** — 既発行の access_token / id_token が一斉に検証不能になる。
2. **レプリカごとに別鍵** — 水平スケール時、レプリカ A が署名したトークンを
   レプリカ B の `/jwks` では検証できない（JWKS 不一致）。
3. **回転が実行されない** — `rotate()` を呼ぶ経路が無く、`SigningKeyRotated` も発行されない。

つまり仕様（ADR-009 / requirements §12）が約束する鍵ライフサイクルを実装が満たしていなかった。

## 決定

1. **`PostgresKeyStore` を追加** — `signing_keys` テーブル
   (`spec/migrations/0001_init.sql`) を唯一の鍵ソースとし、全レプリカが共有する。
   秘密鍵は `private_jwk`(JSONB) に保存（サンプル simplification）。
2. **single-active 不変条件を DB で強制** — `0002_signing_keys_single_active.sql` で
   部分一意インデックス `WHERE active` を張る。これにより
   - 起動時シード: `INSERT ... ON CONFLICT DO NOTHING` で「最初の 1 つ」だけが active
   - `rotate()`: トランザクション内で旧 active を倒して新 active を挿入
   を複数レプリカ同時でも競合安全に行える。
3. **回転は usecase + 監査イベント** — `rotateSigningKeyUseCase` が `keyStore.rotate()` を
   呼び `SigningKeyRotated`(newKid / previousKid) を発行する。永続化非依存なので
   アプリケーション層に置く。
4. **回転の運用エントリポイント** — `infra/scripts/rotate-signing-key.ts`
   (`bun run rotate:key`)。K8s CronJob / scheduled workflow から 90 日ごと、
   または鍵漏洩時の緊急回転に呼ぶ。
5. **検証オーバーラップ** — 旧鍵は inactive として残し `/jwks` と `findByKid` で引ける。
   旧鍵署名トークンの寿命が尽きるまで検証可能（ADR-009 の 7 日オーバーラップ）。

## 影響

- `main.ts` の `assemble()` が persistence モードに応じて KeyStore を選ぶ
  （memory → `InMemoryKeyStore`、postgres → `PostgresKeyStore`）。`src/*` と
  `adapters/http/*` は KeyStore ポートしか知らないため無変更（ADR-003 の実コード証明）。
- `persistence-contract.test.ts` の KeyStore 契約が InMemory と Postgres の両方を
  同一テストで検証する。
- import 済み鍵オブジェクトは kid をキーにキャッシュし、JWK→KeyLike 変換コストを抑える。

## KMS / HSM への発展（ADR-009 の延長）

production では `private_jwk` カラムを「KMS 内の鍵参照 ID」に置換し、署名を KMS API 経由で
行う設計が望ましい。署名鍵オブジェクトはポート層で `unknown` 扱いのため、この差し替えは
`PostgresKeyStore`（あるいは新規 `KmsKeyStore`）内に閉じ、上位層は無変更で済む。

## 却下した代替案

- **getActiveKey / getAllKeys を長時間キャッシュ**: 回転後の伝播が遅れ、レプリカ間で
  新鍵署名トークンの検証窓がずれる。クエリは都度実行し、import 結果のみキャッシュする。
- **single-active をアプリで保証**: レプリカ同時起動・同時回転で破れる。DB の部分一意
  インデックスで強制する。
