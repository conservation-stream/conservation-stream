FROM node:24-slim AS base

RUN corepack enable && corepack prepare pnpm@10.28.1 --activate

FROM base AS deps

WORKDIR /app

COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY packages/mtx-agent/package.json ./packages/mtx-agent/
COPY packages/mtx-manager/package.json ./packages/mtx-manager/
COPY packages/mtx-client/package.json ./packages/mtx-client/
COPY apps/site/api/package.json ./apps/site/api/

RUN pnpm install --frozen-lockfile

FROM base

WORKDIR /app

COPY --from=deps /app ./
COPY packages/mtx-agent ./packages/mtx-agent
COPY packages/mtx-manager ./packages/mtx-manager
COPY packages/mtx-client ./packages/mtx-client
COPY apps/site/api ./apps/site/api

WORKDIR /app/packages/mtx-agent

CMD ["pnpm", "start"]
