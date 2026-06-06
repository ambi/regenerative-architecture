FROM oven/bun:1.3.14-alpine AS ui-build
WORKDIR /src/ra-idp-go/ui
COPY ra-idp-go/ui/package.json ra-idp-go/ui/bun.lock ./
RUN bun install --frozen-lockfile
COPY ra-idp-go/ui ./
RUN bun run build

FROM golang:1.26-alpine AS build
WORKDIR /src/ra-idp-go
COPY ra-idp-go/go.mod ra-idp-go/go.sum ./
RUN go mod download
COPY ra-idp-go .
COPY --from=ui-build /src/ra-idp-go/ui/dist ./ui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ra-idp-go ./cmd/ra-idp-go && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/ra-idp-relay ./cmd/ra-idp-relay

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/ra-idp-go /app/ra-idp-go
COPY --from=build /out/ra-idp-relay /app/ra-idp-relay
COPY ra-idp/spec /app/spec
COPY ra-idp-go/infra/migrations /app/infra/migrations
EXPOSE 8080
ENTRYPOINT ["/app/ra-idp-go"]
