# ADR-053: エージェントランタイム向け workload identity federation を確定する

## ステータス

提案 (draft)。[[wi-54-workload-identity-federation-spiffe]] の意思決定を先行して起草する。
wi-54 の実装着手とともに「採用」へ移す。[[ADR-048]] (エージェント一級プリンシパル)・
[[ADR-049]] (token exchange による委譲・代行)・[[ADR-008]] (client 認証方式)・
[[ADR-023]] (private_key_jwt の検証)・[[ADR-034]] (テナントスコープ永続化) を前提に、
外部 workload attestation を ra-idp の token に federation し、**エージェントランタイムが
長期シークレットなしで資格情報を得る信頼境界**を確定する。本 ADR は自律ワークロード
([[wi-49-agent-identity-first-class-principal]]) の発行経路を、token exchange
([[wi-50-token-exchange-delegation-actor-chain]]) を再利用して実現し、即時失効
([[wi-58-continuous-access-evaluation-agent-revocation]]) と接続する。

## コンテキスト

自律エージェントは人間の介在なしにコンテナ / 関数 / VM 上で起動して行動するため、
資格情報を埋め込めない。長期シークレット (static key / client_secret) をランタイムに
配ると、漏洩・棚卸し困難・ローテーション漏れという non-human identity 最大のリスクを抱える。
現代の基盤は実行環境そのものの attestation を信頼の起点とする。Kubernetes は
projected ServiceAccount token を、クラウドは instance identity document を、
SPIFFE/SPIRE (CNCF) は SVID を発行し、Google Cloud の Workload Identity Federation は
これらを IdP token と交換する。いずれも「ランタイムが置かれた環境を証明させ、その証明と
引き換えに短命トークンを渡す」モデルで、static key を排除する。

ra-idp は `client_credentials` ([[ADR-008]]) を持つが、これは依然として共有シークレットを
要する。外部 attestation を信頼の起点にした federation 経路は存在しない。一方で外部の
証明を受け入れる以上、検証を誤れば任意のワークロードがエージェント資格情報を得てしまう。
そこで「どの発行者を信頼するか」「外部 subject をどの ra-idp プリンシパルへ写すか」
「何を検証すれば交換を許すか」を、既存の token exchange 基盤 ([[ADR-049]]) と JWT/JWKS
検証 ([[ADR-023]]) の上に**保証義務 (fail-closed)** として確定する必要がある。

## 決定

1. **外部 attestation を OIDC 互換 JWT を起点に信頼する**。受理する subject token は
   OIDC 互換の JWT を第一級とし、Kubernetes の projected ServiceAccount token・クラウドの
   instance identity (OIDC) token・SPIFFE の JWT-SVID を対象とする。X.509-SVID (mTLS)
   ベースの bootstrap はより重く、まず JWT で federation を成立させ、X.509 は将来の拡張とする。

2. **trust domain / issuer を登録し、外部 subject から ra-idp principal への mapping を定義する**。
   `WorkloadIdentityProvider` として trust domain / issuer / JWKS 取得元 / 受理する audience を
   登録し、登録済みの issuer のみを信頼する。`SubjectMapping` で外部 subject (k8s の
   `system:serviceaccount:ns:sa`、SPIFFE ID、cloud principal) を ra-idp の [[ADR-048]]
   `Agent` (およびそれに束縛された `OAuth2Client`) へ写す。mapping にない subject は受理しない。

3. **token exchange grant ([[ADR-049]], RFC 8693) を交換機構として再利用する**。専用の資格情報
   経路を新設しない。外部 workload token を `subject_token` として `/token` の
   `grant_type=urn:ietf:params:oauth:grant-type:token-exchange` に提示させ、workload 系の
   `subject_token_type` を受理する。検証を通れば mapping 先の `Agent` プリンシパルに紐づく
   ra-idp token を発行する。audience 限定・短命既定など [[ADR-049]] の交換規律をそのまま継承する。

4. **外部発行者を fail-closed で検証する**。external issuer の JWKS を取得し、署名・`iss`・
   `aud`・`exp` (および必要に応じ `nbf` / 発行者固有 claim) をすべて満たす場合のみ交換する。
   JWKS 取得・署名検証は [[ADR-023]] の private_key_jwt 検証基盤を流用する。登録済み trust
   domain に属さない issuer、検証できない署名、期限切れ・audience 不一致は一律拒否する。
   検証経路に判定漏れがあっても「交換しない」側へ倒す。

5. **発行は短命トークンに限定し、long-lived 資格情報を作らない**。federation 経由で発行する
   ra-idp token は短命既定とし、refresh token を伴わない。workload の継続稼働は再 attestation
   による再交換を要する。これにより資格情報を時間的に bounded に保ち、即時失効
   ([[wi-58-continuous-access-evaluation-agent-revocation]]) を効かせやすくする。federation の
   存在意義 (シークレットレス) を long-lived な発行で台無しにしない。

6. **federation provider と subject mapping を tenant-scoped にする**。`WorkloadIdentityProvider` /
   `TrustDomain` / `SubjectMapping` の登録・参照・操作は tenant-scoped とし ([[ADR-034]])、
   ある tenant に登録した issuer が別 tenant のエージェントを発行できないようにする。
   cross-tenant な federation 信頼は認めない。

7. **ra-idp は relying party / federation 側に徹し、SPIRE を同梱・運用しない**。SPIFFE/SPIRE の
   server / agent を ra-idp に取り込まない。ra-idp は外部の attestation 発行者を信頼し検証して
   token を交換する側であり、attestation を発行するインフラは利用者側の責務とする。

8. **観測と監査**。`WorkloadIdentityProviderConfigured` / `WorkloadTokenExchanged` /
   `WorkloadAttestationRejected` を emit し ([[ADR-018]])、拒否理由 (未登録 issuer・署名不正・
   期限切れ・mapping なし) を残す。federation provider の管理は新規 permission
   `AdminWorkloadIdentityManage` で保護する。

## 影響

- 新規 model `WorkloadIdentityProvider` / `TrustDomain` / `AttestationClaim` / `SubjectMapping` と
  対応する tenant-scoped な Postgres テーブル ([[ADR-034]]) が加わる。
- `/token` の token-exchange 経路 ([[ADR-049]]) が workload 系 `subject_token_type` を受理するよう
  拡張される。交換機構そのものは再利用で、検証段に external issuer の attestation 検証が挿入される。
- external issuer の JWKS 取得・キャッシュ・検証が [[ADR-023]] の検証基盤を共用する形で追加される。
- 管理 UI / API に federation provider registry (issuer 登録・JWKS 設定・subject mapping・無効化) が加わる。
- エージェントランタイムは client_secret を持たずに起動でき、シークレット棚卸し・ローテーションの
  運用負荷が消える。発行は短命のため、継続稼働には再 attestation 経路が前提になる。

## 却下した代替案

- **エージェントランタイムごとに long-lived な `client_secret` を配る**: 実装は最小だが、
  static key の埋め込み・漏洩・棚卸し困難という、まさに federation が排除しようとしている
  シークレットスプロールを温存する。non-human identity の最大リスクを残すため採らない。
- **SPIRE server / agent を ra-idp に同梱・運用する**: attestation 発行まで抱え込むと scope creep に
  なり、ra-idp の責務 (relying party / federation 側) を逸脱する。attestation 発行インフラは
  利用者側に委ね、ra-idp は信頼・検証・交換に徹する。
- **X.509-SVID / mTLS bootstrap を先に実装する**: 証明書配布・回転・mTLS 終端を伴い重い。
  まず OIDC 互換 JWT で federation を成立させ、X.509 は将来の拡張とする方が段階的で安全。
- **issuer 登録なしの trust-on-first-use を許す**: 初出の issuer を自動信頼すると、任意の
  ワークロードがエージェント資格情報を mint できてしまう。登録済み trust domain / issuer のみを
  受理し ([[ADR-049]] / fail-closed)、未登録は一律拒否する。
- **token exchange とは別の専用 federation 資格情報経路を新設する**: 交換規律 (audience 限定・
  短命・actor 表現) を二重に実装することになり、[[ADR-049]] と乖離して攻撃面と保守負荷が増える。
  RFC 8693 の交換機構を subject token 種別の差し替えで再利用する。
