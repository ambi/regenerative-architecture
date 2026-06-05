# OAuth2 IdP — Layer 5 Runtime (ADR-019)
#
# multi-stage build:
#   1. builder: Bun でインストール (devDeps 含む)
#   2. runtime: distroless cc + Bun static binary + アプリ
#
# - non-root (nonroot, UID 65532)
# - read-only rootfs を Kubernetes 側で強制
# - multi-arch (amd64 + arm64) build (buildx)

ARG BUN_VERSION=1.3.14

# ===============================================================
# Stage 1 — Builder
# ===============================================================
FROM oven/bun:${BUN_VERSION} AS builder

WORKDIR /app

# lock first → cacheable deps layer
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile

# Copy source
COPY tsconfig.json ./
COPY main.ts ./
COPY src ./src
COPY adapters ./adapters
COPY spec ./spec
COPY infra ./infra
COPY decisions ./decisions

RUN bun run typecheck
# 本サンプルでは bun run main.ts そのままを使う (TS のまま)。
# 厳密にバンドルしたい場合は: bun build main.ts --target=bun --outfile=dist/server.js

# ===============================================================
# Stage 2 — Bun binary
# ===============================================================
FROM oven/bun:${BUN_VERSION}-distroless AS runtime

# distroless にはシェルが無い。Bun の entrypoint をそのまま使う。
WORKDIR /app

COPY --from=builder /app /app

# Provided by distroless image (UID 65532 nonroot)
USER nonroot

ENV NODE_ENV=production
ENV PORT=3000
EXPOSE 3000

# シグナルハンドリングは main.ts 側 (SIGTERM / SIGINT) で行う。
ENTRYPOINT ["bun", "run", "main.ts"]
