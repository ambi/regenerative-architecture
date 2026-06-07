FROM oven/bun:1.3.14-alpine AS build
WORKDIR /src
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile
COPY . .
RUN bun run build

FROM caddy:2.10-alpine
COPY Caddyfile /etc/caddy/Caddyfile
COPY --from=build /src/dist /srv
EXPOSE 8080
