# Build stage: install dependencies.
FROM node:24-alpine AS deps

WORKDIR /app
COPY apps/web/package.json apps/web/package-lock.json ./
RUN npm ci

# Build stage: production build with standalone output.
FROM node:24-alpine AS builder

WORKDIR /app
# Required at build time; the build fails without it. Inlined into the bundle.
ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL

COPY --from=deps /app/node_modules ./node_modules
COPY apps/web/ ./
COPY --from=deps /app/package-lock.json ./package-lock.json

RUN npm run build

# Runtime stage: only the standalone output (server.js + minimal node_modules)
# plus static assets. Runs as the unprivileged node user.
FROM node:24-alpine

WORKDIR /app
ENV NODE_ENV=production
ENV HOSTNAME=0.0.0.0
EXPOSE 3000

COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

USER node
CMD ["node", "server.js"]
