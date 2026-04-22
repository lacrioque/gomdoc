---
title: Deployment Examples
author: gomdoc
tags: deployment, docker, nginx, traefik
---

# Deployment Examples

These examples show common ways to run gomdoc behind Docker, NGINX, and Traefik.

Replace:

- `docs.example.com` with your public hostname
- `/srv/docs` with the directory that contains your markdown files
- OAuth2 provider values with the values from your identity provider

If OAuth2 is enabled behind a reverse proxy, set `-oauth2-redirect-url` to the public HTTPS callback URL that users see in their browser, for example `https://docs.example.com/oauth2/callback`.

## Docker

### Simple Docker Run

```bash
docker run -d \
  --name gomdoc \
  --restart unless-stopped \
  -p 7331:7331 \
  -v /srv/docs:/docs:ro \
  markusfluer/gomdoc:latest \
  -dir /docs \
  -title "Project Docs"
```

### Docker Run With OAuth2

Create an environment file:

```dotenv
GOMDOC_OAUTH2_CLIENT_ID=client-id
GOMDOC_OAUTH2_CLIENT_SECRET=client-secret
GOMDOC_OAUTH2_COOKIE_SECRET=replace-with-a-long-random-secret
```

Run gomdoc:

```bash
docker run -d \
  --name gomdoc \
  --restart unless-stopped \
  --env-file .env \
  -p 7331:7331 \
  -v /srv/docs:/docs:ro \
  markusfluer/gomdoc:latest \
  -dir /docs \
  -title "Project Docs" \
  -oauth2-auth-url https://accounts.google.com/o/oauth2/v2/auth \
  -oauth2-token-url https://oauth2.googleapis.com/token \
  -oauth2-redirect-url https://docs.example.com/oauth2/callback \
  -oauth2-userinfo-url https://openidconnect.googleapis.com/v1/userinfo \
  -oauth2-scopes "openid,email,profile" \
  -oauth2-allowed-domains example.com
```

### Docker Compose

```yaml
services:
  gomdoc:
    image: markusfluer/gomdoc:latest
    restart: unless-stopped
    ports:
      - "7331:7331"
    volumes:
      - /srv/docs:/docs:ro
    env_file:
      - .env
    command:
      - -dir
      - /docs
      - -title
      - Project Docs
      - -oauth2-auth-url
      - https://accounts.google.com/o/oauth2/v2/auth
      - -oauth2-token-url
      - https://oauth2.googleapis.com/token
      - -oauth2-redirect-url
      - https://docs.example.com/oauth2/callback
      - -oauth2-userinfo-url
      - https://openidconnect.googleapis.com/v1/userinfo
      - -oauth2-scopes
      - openid,email,profile
      - -oauth2-allowed-domains
      - example.com
```

## NGINX

Run gomdoc on localhost and let NGINX terminate TLS:

```bash
gomdoc -dir /srv/docs -port 7331 -title "Project Docs"
```

Example NGINX server block:

```nginx
server {
    listen 80;
    server_name docs.example.com;

    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name docs.example.com;

    ssl_certificate /etc/letsencrypt/live/docs.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/docs.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:7331;
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # MCP uses SSE. Disable buffering so events are delivered immediately.
    location /mcp/ {
        proxy_pass http://127.0.0.1:7331;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_request_buffering off;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

With OAuth2 behind NGINX, use the public redirect URL:

```bash
gomdoc \
  -dir /srv/docs \
  -port 7331 \
  -oauth2-redirect-url https://docs.example.com/oauth2/callback \
  -oauth2-auth-url https://accounts.google.com/o/oauth2/v2/auth \
  -oauth2-token-url https://oauth2.googleapis.com/token \
  -oauth2-userinfo-url https://openidconnect.googleapis.com/v1/userinfo \
  -oauth2-scopes "openid,email,profile" \
  -oauth2-allowed-domains example.com
```

## Traefik

This example assumes Traefik is already running with:

- Docker provider enabled
- `websecure` entrypoint configured
- `letsencrypt` certificate resolver configured
- External Docker network named `proxy`

```yaml
services:
  gomdoc:
    image: markusfluer/gomdoc:latest
    restart: unless-stopped
    networks:
      - proxy
    volumes:
      - /srv/docs:/docs:ro
    env_file:
      - .env
    command:
      - -dir
      - /docs
      - -title
      - Project Docs
      - -oauth2-auth-url
      - https://accounts.google.com/o/oauth2/v2/auth
      - -oauth2-token-url
      - https://oauth2.googleapis.com/token
      - -oauth2-redirect-url
      - https://docs.example.com/oauth2/callback
      - -oauth2-userinfo-url
      - https://openidconnect.googleapis.com/v1/userinfo
      - -oauth2-scopes
      - openid,email,profile
      - -oauth2-allowed-domains
      - example.com
    labels:
      - traefik.enable=true
      - traefik.http.routers.gomdoc.rule=Host(`docs.example.com`)
      - traefik.http.routers.gomdoc.entrypoints=websecure
      - traefik.http.routers.gomdoc.tls=true
      - traefik.http.routers.gomdoc.tls.certresolver=letsencrypt
      - traefik.http.services.gomdoc.loadbalancer.server.port=7331

networks:
  proxy:
    external: true
```

If gomdoc is not the only service on the Docker network, keep the explicit `traefik.http.services.gomdoc.loadbalancer.server.port=7331` label so Traefik routes to the correct internal port.
