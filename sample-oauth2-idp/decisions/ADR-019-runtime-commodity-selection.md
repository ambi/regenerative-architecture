# ADR-019: Runtime / Infra commodity は distroless container + Kubernetes + GitHub Actions

## ステータス

採用

## コンテキスト

REGENERATIVE_ARCHITECTURE.md §3.5 では Runtime & Infrastructure は
「原則としてコモディティ」と述べる。ただし「コモディティ化」とは
「思想を持ち込まない」ではなく「**選択の理由を ADR に残し、いつでも差し替え可能にする**」
ことである。

本サンプルの Runtime 層は次の 4 つを選ぶ:

1. **コンテナ実行環境**
2. **コンテナオーケストレーション**
3. **CI/CD パイプライン**
4. **シークレット管理**

それぞれの選定理由を本 ADR で明示する。

## 決定

### 1. Bun → distroless container

- **ベース**: `gcr.io/distroless/cc-debian12` (TLS / glibc 提供)
- **Bun を bundle**: 公式 Bun の linux x86_64 / arm64 バイナリを multi-stage で組み込む
- **non-root**: UID 65532 (nonroot) で起動
- **read-only rootfs**: `/tmp` のみ tmpfs

理由:
- distroless は CVE 表面積が極小 (busybox 系ですら除外)
- non-root + read-only でコンテナエスケープ難度を上げる
- multi-arch (amd64 + arm64) ビルド対応

### 2. Kubernetes (Vanilla)

- **manifest**: Kustomize base + overlays (dev / prod)
- **deployment**: 最低 2 replicas、PDB で同時停止 1 まで
- **HPA**: CPU + custom metric (`oauth2_token_requests_total` rate)
- **NetworkPolicy**: default-deny + ingress/egress を明示
- **Pod Security Standards**: `restricted` プロファイル

理由:
- ベンダ中立 (AWS EKS / GCP GKE / Azure AKS のいずれでも動く)
- 業界標準のため運用人材の流動性が高い

代替案:
- ECS / Cloud Run: ベンダロックイン。スケールは速いがマルチクラウド戦略と相性が悪い
- Nomad: シェアが小さく運用知識が散逸している

### 3. GitHub Actions

- **ci.yaml**: PR ごと (typecheck + 全テスト + drift 検知 + 脆弱性スキャン + container build)
- **nightly.yaml**: 毎晩 (k6 + OIDC conformance suite)
- **release.yaml**: タグプッシュ (cosign + SBOM + SLSA)

理由:
- GitHub をすでに使っているチームにとって追加コストが最小
- GitHub OIDC で keyless cosign 署名が可能 (long-lived secret なし)
- Linux + Mac + Windows ランナーを混在実行可能

代替案:
- Jenkins: メンテナンス負荷が高い
- CircleCI / GitLab: 別 SaaS 依存

### 4. シークレット管理

- 本サンプル: dev は `.env`、prod 想定は External Secrets Operator → AWS Secrets Manager
- 署名鍵: ADR-009 通り KMS / HSM 想定 (本サンプルは in-memory)

シークレットは本 ADR ではなく ADR-009 (鍵管理) と Phase 3 の k8s manifest に集約する。

## 却下した代替案 (Runtime 全般)

- **AWS Lambda などのサーバーレス**: cold start レイテンシが SLO に響く (`token` p99 = 300ms)。
  warm pool を維持するコストが Kubernetes と変わらなくなる。
- **VM ベース (cloud-init)**: スケーリング応答性が悪い。
- **Docker Swarm**: メンテナンスが事実上停止。

## 影響

- `infra/docker/Dockerfile` — multi-stage build (builder → distroless)
- `infra/docker/docker-compose.dev.yaml` — フルスタック開発環境
- `infra/k8s/base/` + `infra/k8s/overlays/{dev,prod}/`
- `.github/workflows/{ci,nightly,release}.yaml`
- `Bun` を Runtime とすることはここで固定。後日 Node.js / Deno に切り替える際は
  本 ADR の派生 ADR (ADR-019a) を起こす。

## 関連

- ADR-009 (Key rotation — KMS / HSM)
- ADR-020 (Supply chain — SLSA / Sigstore / SBOM)
- ADR-021 (Progressive delivery — Argo Rollouts)
- REGENERATIVE_ARCHITECTURE.md §3.5, §6
