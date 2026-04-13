---
title: "Deploy"
weight: 15
---

# Deploy

ChatAPI is a single binary with SQLite included. No external services required.

---

## Docker Compose

Create a `docker-compose.yml` on your server — no need to clone the repository:

```yaml
services:
  chatapi:
    image: hastenr/chatapi:latest
    ports:
      - "8080:8080"
    environment:
      JWT_SECRET: ${JWT_SECRET}
      ALLOWED_ORIGINS: ${ALLOWED_ORIGINS}
      WEBHOOK_URL: ${WEBHOOK_URL:-}
      WEBHOOK_SECRET: ${WEBHOOK_SECRET:-}
    volumes:
      - chatapi-data:/data
    restart: unless-stopped

volumes:
  chatapi-data:
```

Create a `.env` file alongside it:

```env
JWT_SECRET=your-secret-here
ALLOWED_ORIGINS=https://yourapp.com
WEBHOOK_URL=https://yourapp.com/api/chatapi/webhook
WEBHOOK_SECRET=your-webhook-secret
```

Start:

```bash
docker compose up -d
```

```bash
curl http://localhost:8080/health
# {"status":"ok","db_writable":true}
```

---

## Single binary

Download the latest binary for your platform from [GitHub Releases](https://github.com/getchatapi/chatapi/releases):

```bash
# Linux (amd64)
curl -L https://github.com/getchatapi/chatapi/releases/latest/download/chatapi-linux-amd64 \
  -o chatapi && chmod +x chatapi

export JWT_SECRET=your-secret-here
export ALLOWED_ORIGINS=https://yourapp.com
./chatapi
```

SQLite data is written to `./chatapi.db` by default. Set `DATABASE_DSN` to control the path:

```env
DATABASE_DSN=file:/var/lib/chatapi/chatapi.db
```

---

## Reverse proxy

Run ChatAPI behind nginx or Caddy. WebSocket upgrade must be proxied correctly.

**nginx:**

```nginx
server {
    listen 443 ssl;
    server_name chat.yourapp.com;

    location / {
        proxy_pass         http://localhost:8080;
        proxy_http_version 1.1;
        proxy_set_header   Upgrade $http_upgrade;
        proxy_set_header   Connection "upgrade";
        proxy_set_header   Host $host;
        proxy_set_header   X-Real-IP $remote_addr;
    }
}
```

**Caddy:**

```
chat.yourapp.com {
    reverse_proxy localhost:8080
}
```

Caddy handles WebSocket upgrade and TLS automatically.

---

## Data persistence

SQLite data lives at `/data/chatapi.db` in the Docker image. Always mount a named volume or host directory:

```yaml
volumes:
  - chatapi-data:/data   # named volume (recommended)
  # or
  - /var/lib/chatapi:/data  # host path
```

Back up by copying the `.db` file while the server is running — SQLite WAL mode is safe for live copies.
