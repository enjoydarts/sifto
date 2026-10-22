# ── Dev stage ───────────────────────────────────────────────
FROM node:22.23.4-alpine AS dev
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
EXPOSE 3000
ENV WATCHPACK_POLLING=true
CMD ["npm", "run", "dev", "--", "--hostname", "0.0.0.0"]

# ── Production build ─────────────────────────────────────────
FROM node:22.23.4-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm ci

FROM node:22.23.4-alpine AS builder
ARG NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY
ENV NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY=${NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY}
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN npm run build

FROM node:22.23.4-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
RUN apk upgrade --no-cache zlib
RUN rm -rf /usr/local/lib/node_modules/npm /usr/local/bin/npm /usr/local/bin/npx
COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
RUN mkdir -p /app/.next/cache && chown -R node:node /app/.next
ENV HOME=/home/node
USER 1000:1000
EXPOSE 3000
CMD ["node", "server.js"]
