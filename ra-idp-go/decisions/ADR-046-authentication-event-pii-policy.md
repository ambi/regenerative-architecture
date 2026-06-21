# ADR-046: 認証イベントの PII 取り扱いポリシー

## ステータス

採用。[[wi-44-authentication-event-store-and-search]]。[[ADR-018]] (監査 / アプリログ分離)・
[[ADR-029]] (sentinel hash)・[[ADR-045]] (保持期間) を前提に、認証イベントが抱える PII
(IP / username / 位置 / デバイス) の**保管形式**を確定する。

## コンテキスト

認証イベントは調査価値の高い識別情報を含むが、そのまま平文で長期保管すると監査ログ自体が
情報漏洩の標的になる。一方で調査には「同じ IP / 同じユーザからの試行をまとめる」相関能力が
要る。相関に足る最小限を hash / truncation で持ち、平文は本当に必要な範囲・期間に限定する。

## 決定

1. **IP アドレス**。2 系統で持つ。
   - **truncated**: IPv4 は /24、IPv6 は /48 に丸めた `ip_truncated`。粗い地理 / ネットワーク
     単位の相関と admin 検索のフィルタに使う。
   - **hash**: tenant salt 付き SHA-256 の `ip_hash`。同一 IP の厳密相関と bucket の `keyHash`
     に使う。生 IP は保管しない。

2. **username**。
   - **hash first-class**: tenant salt 付き SHA-256 の `username_hash` を全イベントで持ち、
     相関 / bucket key とする。
   - **平文は失敗イベント限定 + 7 日**: `fail` 個別イベントに限り平文 username を 7 日だけ
     保持し、以後は sweep で平文列を null 化して粒度を落とす (hash は残る)。成功イベントは
     `sub` を持つため平文 username を保管しない。

3. **位置情報**。**country code のみ first-class** (`country_code`、OSS GeoIP DB の後付け、
   当面 `""` 許容)。市区町村・緯度経度などの細かい位置は保管しない。

4. **device fingerprint**。**hash 保管のみ** (`device_fingerprint_hash`)。raw の fingerprint
   文字列は保存しない。

5. **impersonation は短縮不可・平文据え置き**。impersonation イベントは [[ADR-045]] の通り
   retention 短縮の対象外であり、本人保護に必要な actor / target は据え置く。

6. **レビュー運用**。hash / truncation を誤ると個人情報が監査ログに流れる。本 ADR の表に
   照らし、認証イベントの PII 列を追加 / 変更する変更は**レビュアー 2 名以上**で確認する。

| 種別               | 保管形式                          | 平文保持        |
| ------------------ | --------------------------------- | --------------- |
| IP                 | /24・/48 truncated + SHA-256 hash | なし            |
| username           | SHA-256 hash (first-class)        | 失敗のみ 7 日   |
| 位置               | country code のみ                 | (細粒度は無し)  |
| device fingerprint | SHA-256 hash                      | なし            |

## 影響

- bucket の `keyHash` は本 ADR の username/IP hash と同一方式を使い、tenant salt により
  cross-tenant で集約しない ([[ADR-041]])。
- 平文 username の 7 日縮約は [[ADR-045]] の sweep が担う (列の null 化)。
- `country_code` が `""` のままでも UI / 検索は動く (GeoIP 連携は別 WI)。

## 却下した代替案

- **生 IP / 生 username を保管**: 相関は楽だが監査ログが漏洩の標的になり PII 規制にも反する。
- **hash のみ・truncated を持たない**: 粗い地理 / ネットワーク単位のフィルタ検索ができず、
  admin の調査体験が劣化する。truncated と hash を併用して相関と検索性を両立する。
