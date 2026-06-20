# RA Identity UI

`ra-idp-go` の認証 UI は、エンタープライズ向けの認証・ID 管理画面として、
モダンで適切、使いやすく、視覚的にも高品質であることを目標とする。

## デザイン指針

- **信頼性を最優先する。** 落ち着いた配色、明確なサービス識別、現在の操作と
  セキュリティ状態の説明により、利用者が安心して認証判断を行えるようにする。
- **判断に必要な情報を先に示す。** 画面タイトル、要求元、共有情報、次に起きること、
  取り消し方法を簡潔な情報階層で提示する。
- **重要な操作を迷わせない。** 主操作は 1 画面につき原則 1 つとし、拒否・取消操作と
  視覚的に区別する。OAuth/OIDC のフォーム名、送信値、遷移契約は UI 都合で変更しない。
- **アクセシビリティを標準品質とする。** キーボード操作、可視フォーカス、十分な
  コントラスト、明示的なラベル、適切な `aria-*`、モーション抑制設定に対応する。
- **企業利用に耐える密度を保つ。** 過剰な装飾や消費者向けの演出を避け、余白、
  タイポグラフィ、境界線、状態色を一貫して使う。
- **レスポンシブでも内容を欠落させない。** デスクトップでは補足情報を活用し、
  モバイルでは認証操作を優先しつつ、サービス識別と安全上の注意を維持する。
- **共通部品で一貫性を保つ。** Tailwind CSS、Radix UI、shadcn/ui 形式のローカル部品を
  基盤とし、色・角丸・フォーカス・disabled 状態を各画面で個別実装しない。

## 管理コンソールの方針

管理コンソールは、Keycloak、Okta、Google Cloud IAM に共通するディレクトリ中心の
情報設計を参考にする。左ナビゲーションで管理対象を明示し、一覧では検索・状態・
主要な権限を高密度に比較でき、選択した主体の詳細と変更操作を同じコンテキスト内に
表示する。作成や無効化など影響の大きい操作は通常の参照操作から視覚的に分離する。

- 一覧を作業の起点とし、検索、フィルター、状態、MFA、ロールを一目で確認できること。
- 詳細ペインで主体ID、認証状態、権限を確認してから変更できること。
- ロールなどの権限変更はインライン編集にせず、専用画面で追加・削除差分を確認してから
  確定すること。
- 危険操作は明確な説明と状態色を使い、通常操作と取り違えないこと。
- クライアント作成時の secret は一度だけ表示し、削除は client ID と影響を確認してから
  確定すること。
- 将来のグループ、アプリケーション、監査ログ追加を想定した一貫したナビゲーションを
  提供するが、未実装機能を操作可能に見せないこと。
- 管理画面は `AdminShell` でヘッダ、sidebar、breadcrumb、本文幅、操作位置を統一する。
- 未認証で `/admin/*` を直リンクした場合は `/login` へ移動し、認証後に同じ管理画面へ
  復帰する。戻り先は現在の realm の `/admin` 配下だけを許可する。

参考:
[Keycloak Server Administration Guide](https://www.keycloak.org/docs/latest/server_admin/),
[Okta Manage users](https://help.okta.com/en-us/content/topics/users-groups-profiles/usgp-people.htm),
[Google Cloud IAM access management](https://cloud.google.com/iam/docs/granting-changing-revoking-access)

## UI ライブラリ選定

UI 基盤は、アクセシビリティとデザインの統制を両立し、特定の完成済みテーマへ
過度に依存しない構成とする。

| ライブラリ | 役割 | 選定理由 |
| --- | --- | --- |
| React + TypeScript | UI と型安全な画面実装 | 小さな認証画面から将来の管理画面まで、状態とコンポーネント境界を明確に保てる |
| Vite | 開発サーバーと production build | GatewayやCDNから配信する独立した静的bundleを高速かつ単純に生成できる |
| Tailwind CSS | デザイントークンとスタイリング | 独自の企業向けブランド表現を維持しながら、状態・レスポンシブ・アクセシビリティを一貫して実装できる |
| Radix UI | アクセシブルな headless primitive | キーボード操作や ARIA を備えた振る舞いを、見た目と分離して利用できる |
| shadcn/ui 形式のローカル部品 | Button、Input、Label、Card、Alert など | ソースをリポジトリで所有し、認証画面の要件に合わせて監査・変更できる。外部テーマへの実行時依存を増やさない |
| TanStack Router | 画面ルーティング | Go から渡される page data と画面種別の対応を型安全に管理できる |
| TanStack Table | 将来の管理データグリッド | 利用者・クライアント・セッション等の一覧で、sorting、filtering、pagination を UI 表現から分離できる。現在の認証 4 画面では未使用 |
| Tabler Icons | アイコン | 一貫した線幅と十分な種類を持ち、装飾ではなく状態や操作の補助として使える |
| class-variance-authority / clsx / tailwind-merge | variant と class の合成 | 共通部品の状態表現を型安全にし、Tailwind class の競合を局所的に解決できる |
| Biome | lint と format | UI コードと CSS の品質基準を高速に自動検証できる |

選定時は、アクセシビリティ、bundle サイズ、保守性、デザインの所有権、
Go 側の HTTP 契約を壊さないことを優先する。新しい UI ライブラリは、既存基盤で
解決できない具体的な要件がある場合に限って追加する。

フロントエンドとバックエンドの配信境界、認可トランザクション、Cookie、CSRF の設計は
[`ARCHITECTURE.md`](ARCHITECTURE.md) を参照する。

## 実装上の確認事項

UI 変更時は `bun run lint`、`bun run typecheck`、`bun run build` を実行する。
認証APIを変更した場合は Go 側の HTTP E2E テストも実行し、Cookie、CSRF、
OAuthリダイレクト、JSON契約が維持されていることを確認する。

Vite CLI は `#!/usr/bin/env node` を持つが、開発・buildスクリプトではJSエントリを
`bun` に直接渡して実行する。これによりNode.jsを別途要求せず、プロセス表示も
`bun .../vite.js` に統一する。

## E2E スモーク

`bun run test:e2e` で SPA の golden path (`/authorize → login → consent → callback`)
を 1 本のスモークとして検証する。ランナーは `bun test` + 組み込みの `Bun.WebView`
(macOS は WKWebView、Linux/Windows はインストール済み Chrome を CDP 経由で駆動) で、
外部のブラウザ自動化フレームワークや別ブラウザのダウンロードは不要。

テスト本体 (`tests/e2e/`) が以下を自動起動・停止する:

- Go API を `memory` モードで `:8081` に (`ADDR=:8081 ISSUER=http://localhost:5173`、
  ブラウザ origin と一致させて CSRF/origin 検査を通すため)。
- Vite dev server を `:5173` に (`/authorize`・`/api` を 8081 にプロキシ)。
- demo-client の登録済み redirect_uri (`http://localhost:3000/callback`) を受ける
  最小コールバックサーバを `:3000` に。

これにより、SPA dispatcher の画面分岐 (`meta[name="ra-idp:page"]`) と、cross-origin
redirect で `code` / `iss` が落ちないこと (RFC 9207) の 2 領域の回帰を機械検知する。
`go` と `bun` が PATH にあれば追加準備は不要。
