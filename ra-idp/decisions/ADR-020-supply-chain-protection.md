# ADR-020: サプライチェーン保護 — SLSA Level 3 を目指し、Sigstore 署名 + CycloneDX SBOM を成果物に同梱する

## ステータス

採用

## コンテキスト

REGENERATIVE_ARCHITECTURE.md §6 が「変化を本番に届ける能力」を一級設計対象とした。
「AI による攻撃加速」の時代、攻撃者は CI/CD パイプラインや依存ライブラリを攻撃ベクトルとする。
SolarWinds・Codecov・log4j 事例が示すように、コードレベルの脆弱性対策と同等以上の
**サプライチェーン保護**が必要である。

OAuth2 IdP は他システムが認証情報を委ねる対象なので、ここを汚染されると
全システムが連鎖的に侵害される (ピボット攻撃の起点)。
従ってサプライチェーン保護の優先度は最高クラス。

## 決定

### 1. SLSA Level 3 を目標とする

- **L1**: ビルドが文書化されている (README に書く)
- **L2**: ビルドが provenance を生成し、改ざん不可能なストアに保管
- **L3**: ビルドが分離された hosted system 上で実行され、provenance が non-falsifiable
- **L4**: two-person review が要求される (本アプリは out of scope)

SLSA L3 の達成には GitHub-hosted runner + SLSA Generator action を使えばよい。

### 2. Sigstore (cosign) でコンテナ署名

- リリース時に `cosign sign` で署名 (GitHub OIDC token を使った keyless 署名)
- 検証は `cosign verify` で行う。Kubernetes admission controller (Policy Controller)
  で「未署名イメージは deploy 拒否」を強制
- 透明性ログ (Rekor) で全署名が公開される

### 3. CycloneDX SBOM を成果物に同梱

- `bun build` の出力に対し Syft で SBOM を生成
- `cosign attest` で SBOM を attestation として署名
- 攻撃時の「我々のコードに log4j 2.14 は含まれるか?」に 30 秒で答えられる体制

### 4. 依存ロックファイルの再現性検証

- `bun install --frozen-lockfile` を CI で使う (lockfile が更新されると build fail)
- `bun.lock` の checksum を SBOM に含める

### 5. 依存スキャン

- **コンテナイメージ**: Trivy (CVE + secret + config)
- **コード**: CodeQL (semantic SAST)
- **OAuth 特化**: semgrep ruleset (OAuth 2.0 Security BCP 由来の禁止パターン)

## 却下した代替案

- **SLSA Level 1 で十分とする**: provenance が改ざん可能では意味がない
- **GPG 署名 + Maven Central 風**: keyless でないため鍵漏洩リスクが残る
- **SBOM をリリース後に生成**: 攻撃時に「現在動いている版」と「最新の SBOM」が
  乖離する可能性。リリースと SBOM は atomic に紐付ける

## 影響

- `.github/workflows/release.yaml` に cosign / syft / SLSA Generator を追加
- Kubernetes overlay に Policy Controller の constraint を追加 (prod のみ)
- 依存追加時は CI で SBOM 自動再生成

## 関連

- REGENERATIVE_ARCHITECTURE.md §6
- ADR-019 (Runtime / CI 選定)
- ADR-021 (Progressive delivery)
- SLSA Framework, Sigstore, CycloneDX
