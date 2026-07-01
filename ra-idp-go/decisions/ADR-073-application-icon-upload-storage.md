# ADR-073: Store application icons as tenant-scoped validated blobs

## ステータス
採用。`spec/contexts/application.yaml` の `models.Application`、`interfaces.UploadApplicationIcon`、`interfaces.DeleteApplicationIcon`、`interfaces.GetApplicationIcon` と `internal/application` に反映。

## コンテキスト
Application はこれまで `icon_url` を自由入力として持っていた。これは管理者に外部 URL の用意を要求し、リンク切れ、任意外部 URL の埋め込み、利用者ポータルからの第三者トラッキングを招く。Application アイコンはテナント境界内の小さい静的アセットであり、初期実装では CDN や外部 object store に依存せず、IdP が検証・保存・配信を所有する必要がある。

画像アップロードは保存型 XSS と content sniffing のリスクがある。特に SVG は画像として見える一方でスクリプトや外部参照を含み得るため、サニタイズなしに受け入れない。

## 決定
1. Application は `icon_object_key` を保存し、`icon_url` は IdP の内部配信 URLとして生成する。管理者は `icon_url` を直接入力しない。
2. 初期保存先は PostgreSQL の `application_icons` テーブルとする。主キーは `(tenant_id, application_id, object_key)` で、`content_type`、`size_bytes`、`data`、`created_at` を保持する。
3. 受理形式は PNG、JPEG、WebP、GIF に限定し、最大サイズは 256 KiB とする。形式はリクエストヘッダーや拡張子ではなく magic byte で判定する。
4. SVG は初期対応から除外する。将来 SVG を受理する場合はサニタイズ方針を別 ADR で決める。
5. 配信レスポンスは保存済み `content_type` を固定で返し、`X-Content-Type-Options: nosniff` と長すぎない private cache を付ける。別テナント、別 Application、削除済み object は未存在として扱う。
6. アイコン差し替えは新しい `object_key` を生成して保存し、Application の `icon_object_key` と `icon_url` を更新する。削除時は Application の参照を空にし、該当 Application の icon blob を削除する。

## 却下した代替案
- 外部 URL 入力を継続する: 管理者負担と外部トラッキング/リンク切れの問題が残る。
- ファイルシステム保存: ローカル開発では簡単だが、複数 replica とバックアップの責務が曖昧になる。
- tenant-scoped object store: 本番運用では自然だが、初期実装に storage service 設定を要求し、デモ IdP の起動容易性を落とす。
- SVG を受理する: サニタイズなしでは保存型 XSS のリスクが高く、初期の安全要件に合わない。

## 影響
- SCL に `Application.icon_object_key`、`ApplicationIconUploadResponse`、`UploadApplicationIcon`、`DeleteApplicationIcon`、`GetApplicationIcon`、`ApplicationIconUpdated` を追加する。
- PostgreSQL schema に `applications.icon_object_key` と `application_icons` を追加する。
- 管理 UI は URL 入力欄をファイル選択/プレビューに置き換える。
- 利用者ポータルと管理画面は従来通り `icon_url` を表示に使うが、その値は内部配信 URL になる。
