FROM node:22.23.3-alpine
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
# Copy only build inputs: local node_modules, .next, and .env stay outside CI.
COPY src ./src
COPY public ./public
COPY scripts ./scripts
COPY tsconfig.json ./
COPY *.ts *.mjs ./
CMD ["sh", "-ec", "npm run lint && npm run build"]
