# ra-idp コンセプション

## 位置づけ

この文書は、ra-idp の開発開始時に存在すべきだったコンセプションを、2026年6月14日に
既存の SCL、ADR、README、実装、テストから遡及復元したものである。当時の原文ではなく、
現存する成果物から一貫して読み取れる構想を記録する。

## 期待する成果

- OAuth 2.0 / OpenID Connect に準拠した Identity Provider を、安全に実装・運用できる。
- 認証・認可の重要な振る舞いを SCL に集約し、実装言語やランタイムを変えても再生成できる。
- 仕様、設計判断、実装、テスト、運用設定の対応関係を追跡できる。
- セキュリティ要件の濃い題材を通じて Regenerative Architecture の実用性を検証できる。

## 主体と境界

- Resource Owner: 自身を認証し、Client へのアクセスを承認する利用者。
- Client / Relying Party: 認可コード、トークン、UserInfo を利用するアプリケーション。
- Administrator: 所属テナント内のユーザーと Client を管理する運用者。
- System Administrator: テナントを管理する運用者。
- Resource Server: Access Token を検証して保護対象 API を提供するシステム。
- ra-idp: Authorization Server / OpenID Provider として振る舞う。

初期の中心範囲は OAuth 2.0 / OpenID Connect とそのセキュリティ拡張である。
SAML、WS-Federation、外部 IdP 連携、SCIM、高保証プロファイルは将来拡張とし、
既存の境界を壊さず追加できる設計を求める。

## 必須事項

- Authorization Code Grant、Refresh Token、Client Credentials、Device Authorization Grant を扱う。
- OIDC Discovery、JWKS、ID Token、UserInfo を提供する。
- 認可コード、PAR request URI、パスワードリセットトークンなどの単一使用を保証する。
- redirect URI は登録値と完全一致させる。
- public / FAPI Client では PKCE S256 を必須とし、Client ごとの方針を仕様化する。
- Refresh Token をローテーションし、再利用検出時に同じファミリーを失効する。
- FAPI Client では PAR とセンダー制約を要求する。
- パスワード、Client Secret、Refresh Token 等の秘密情報を平文保存しない。
- 認可、ユーザー、Client、トークン、永続化をテナント境界内に閉じる。
- 監査イベントとアプリケーションログを分離する。
- SCL と ADR を保存し、実装・テスト・運用設定を再生成または交換可能にする。

## 禁止事項

- Implicit Grant と Resource Owner Password Credentials Grant を提供しない。
- 未登録 redirect URI、認可コードの再利用、Refresh Token の再利用を受理しない。
- Client、ユーザー、トークン、同意をテナント境界を越えて参照・変更しない。
- Client Secret、Refresh Token、パスワード、リセットトークンを平文で永続化しない。
- 認証・認可の規則を HTTP ハンドラや UI だけに埋め込まない。
- AI が生成した実装の自己申告だけで高リスクな変更を承認しない。

## 制約

- 外部標準の採用範囲は `spec/scl.yaml` の `standards` を正とする。
- 重要な設計判断は `decisions/ADR-*.md` に残す。
- 開発時はインメモリアダプタで実行でき、本番向けには PostgreSQL、Redis、Kafka 等へ差し替えられる。
- Go と UI の TypeScript / Bun は現在の実装手段であり、仕様核には含めない。
- UI は日本語と英語、WCAG 2.2 AA を対象とする。

## 優先順位とトレードオフ

1. 認可境界、秘密情報、トークン再利用防止などの安全性。
2. SCL と実装・テストの追跡可能性。
3. アダプタとランタイムの交換可能性。
4. 標準準拠と相互運用性。
5. 性能、運用性、開発者体験。

安全性のために複雑さを増やす場合も、仕様核とアプリケーション論理は単純に保つ。
互換性と安全性が衝突する場合は、Client 種別やプロファイルで段階化し、ADR に理由を残す。

## 具体例と反例

- 例: public Client が PKCE S256 を使って認可コードを一度だけ交換できる。
- 例: Refresh Token の再利用を検出すると、そのファミリーをすべて失効する。
- 例: admin は所属テナントの Client を管理でき、別テナントの Client は管理できない。
- 反例: redirect URI の前方一致やワイルドカード一致を許可する。
- 反例: 期限切れまたは使用済みのトークンを再び受理する。
- 反例: SCL に根拠のない認可分岐を HTTP ハンドラへ追加する。

## 未決定事項と仮定

- SAML、WS-Federation、外部 IdP 連携、SCIM の具体的な優先順位は未決定。
- HSM / KMS、マルチリージョン、認証規格の正式認証は本番化前に別途決定する。
- テナントごとの署名鍵分離、ポリシー上書き、データ所在地は未決定。
- 現在のロードマップは要求の優先順位を示すが、実装済み範囲の規範仕様は SCL を正とする。

## 受け入れの指標

- `go test -race ./...` と `go vet ./...` が成功する。
- UI の `bun run typecheck`、`bun run lint`、`bun run build` が成功する。
- Go の coherence test が SCL 内部参照とバインディングの整合を確認する。
- SCL の保証義務ごとに必要な検証と再検証条件が宣言されている。
- 高リスクな未充足や例外承認が完了レポートに明示される。
