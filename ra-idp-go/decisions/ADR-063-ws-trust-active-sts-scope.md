# ADR-063: WS-Trust active STS の初期対応範囲を確定する

## ステータス

採用 (accepted)。[[wi-62-ws-trust-active-sts]] の意思決定。[[ADR-059]] の claim 発行、
[[ADR-060]] の SAML assertion 署名、[[ADR-062]] の MEX / federation metadata 公開を前提に、
ra-idp-go が WS-Trust 1.3 active requestor STS として扱う最小範囲を確定する。

## コンテキスト

Microsoft 365 などの rich client / legacy active authentication は、ブラウザの WS-Federation
passive profile ではなく SOAP ベースの WS-Trust active endpoint を使う。AD FS 互換の実装では
MEX が endpoint と policy を広告し、`usernamemixed` endpoint が WS-Security UsernameToken を
受けて SAML assertion を RSTR で返す。

WS-Trust は SOAP / WS-Security / WS-Addressing / SAML 署名が重なるため、手広く対応すると
replay・XML wrapping・認証方式混線のリスクが増える。初期対応は Microsoft 365 active sign-in に
必要な Issue binding と UsernameToken に限定する。

## 決定

1. **WS-Trust 1.3 Issue binding のみ対応する。**
   `Validate` / `Renew` / `Cancel` は対象外。`Action` / `RequestType` は
   `http://docs.oasis-open.org/ws-sx/ws-trust/200512/Issue` のみ受理する。

2. **認証方式は `usernamemixed` の UsernameToken のみ対応する。**
   username/password は既存 `UserRepository`、`PasswordHasher`、`LoginAttemptThrottle` を使って
   検証する。Kerberos/IWA の `windowstransport` は本シリーズの範囲外とし、必要なら別 WI とする。

3. **WS-Addressing / WS-Security の必須要素を fail-closed に検証する。**
   `MessageID`、`To`、`Action`、UsernameToken、Timestamp、`AppliesTo` は必須。Timestamp は
   期限切れと大きな未来時刻を拒否し、`MessageID` は短期 replay store に記録する。

4. **`AppliesTo` は登録済み WS-Fed relying party に解決する。**
   未登録対象は拒否する。発行 assertion の Audience / Recipient は解決した RP に束縛し、claim は
   RP の `ClaimMappingPolicy` で発行する。

5. **RSTR は署名済み SAML assertion を SOAP 1.2 で返す。**
   既定 token type は SAML 1.1。RST が SAML 1.1 / SAML 2.0 を明示した場合は対応し、それ以外は拒否する。

## 影響

- `/trust/usernamemixed` が active STS endpoint として動作し、MEX の広告先と一致する。
- WS-Fed passive と WS-Trust active が同じ RP trust、claim mapping、SAML 署名器を共有する。
- `windowstransport` と Hybrid Azure AD Join デバイス登録は提供しないことが明確になる。

## 却下した代替案

- **WS-Trust 全 binding 対応。** Microsoft 365 active sign-in の初期価値に対して攻撃面と検証負荷が大きい。
- **Kerberos `windowstransport` の同時実装。** keytab/SPNEGO/コンピュータアカウント認証は別の信頼境界であり、[[wi-65-kerberos-spnego-inbound-silent-sso]] とも分ける。
- **SOAP/WS-Security の寛容な受理。** 相互運用性よりも replay / audience 混線防止を優先し、必須要素欠落は拒否する。
