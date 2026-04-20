FROM node:20-alpine AS builder

ENV NODE_OPTIONS="--dns-result-order=ipv4first"

ENV npm_config_audit=false \
    npm_config_fund=false \
    npm_config_progress=false \
    npm_config_fetch_retries=5 \
    npm_config_fetch_retry_mintimeout=20000 \
    npm_config_fetch_retry_maxtimeout=120000 \
    npm_config_fetch_timeout=120000

WORKDIR /app

COPY package.json ./
COPY package-lock.json ./
RUN if [ -f package-lock.json ]; then npm ci --no-audit --no-fund; else npm install --no-audit --no-fund; fi

COPY . .
RUN npm run build

RUN if [ -d /app/dist ]; then mv /app/dist /app/out; \
    elif [ -d /app/build ]; then mv /app/build /app/out; \
    else echo "Frontend build output not found (no dist/ or build/)" >&2; exit 1; fi

FROM nginx:1.27-alpine
COPY --from=builder /app/out/ /usr/share/nginx/html/
COPY nginx/default.conf /etc/nginx/conf.d/default.conf
EXPOSE 80 443
CMD ["nginx", "-g", "daemon off;"]
