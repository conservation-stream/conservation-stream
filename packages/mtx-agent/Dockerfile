FROM node:24-slim AS base

RUN corepack enable && corepack prepare pnpm@10.28.1 --activate

FROM base AS deps

WORKDIR /app

COPY pnpm-lock.yaml pnpm-workspace.yaml package.json ./
COPY packages/mtx-agent/package.json ./packages/mtx-agent/
COPY packages/mtx-manager/package.json ./packages/mtx-manager/
COPY packages/mtx-client/package.json ./packages/mtx-client/
COPY apps/site/api/package.json ./apps/site/api/
COPY internal/actions/package.json ./internal/actions/

RUN pnpm install --frozen-lockfile

FROM deps AS build

COPY packages/mtx-agent ./packages/mtx-agent
COPY packages/mtx-manager ./packages/mtx-manager
COPY packages/mtx-client ./packages/mtx-client
COPY apps/site/api ./apps/site/api

RUN pnpm --recursive --filter @conservation-stream/mtx-agent... run build --if-present
RUN pnpm --filter @conservation-stream/mtx-agent --prod --legacy deploy /prod/mtx-agent

FROM base

WORKDIR /app

COPY --from=build /prod/mtx-agent ./

CMD ["pnpm", "start"]
