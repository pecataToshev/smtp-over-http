# smtp-over-http

Two small Go services that let applications send email through an external SMTP server without giving them the SMTP credentials. Useful on platforms that block outbound SMTP ports, or anywhere the credentials should stay outside the application environment.

```
[SMTP client]
     |
     |
     | (SMTP + LOGIN auth)
     |
     v
  smtp2http (internal)
     |
     |
     | HTTPS + Bearer API key
     |
     v
  http2smtp (external)
     |
     |
     | Proper SMTP Connection
     |
     v
[real SMTP server]
```

- `smtp2http`: listens for SMTP, requires LOGIN auth, forwards the **raw** RFC 5322 message over HTTP.
- `http2smtp`: receives the HTTP payload, authenticates via API key, sends via your real SMTP server.

## Docker images

Both services are published to GitHub Container Registry for `linux/amd64` and `linux/arm64`:

```bash
docker pull ghcr.io/pecatatoshev/smtp-over-http/http2smtp:latest
docker pull ghcr.io/pecatatoshev/smtp-over-http/smtp2http:latest
```

| Tag             | Published from                  |
| --------------- | ------------------------------- |
| `latest`        | the `main` branch               |
| `sha-<commit>`  | every push                      |
| `1.2`           | a `v1.2` git tag                |

## Why

- SMTP credentials stay outside the application environment.
- `smtp2http` runs on a private network; only your internal apps can reach it.
- Minimal attack surface: only a single HTTP POST path is exposed publicly on `http2smtp`, protected by a shared secret.

## Security

- **Required SMTP auth on smtp2http**: at least one `SMTP_USERS_<service>=user:pass` variable must be set. Only the LOGIN mechanism is supported.
- **API key between services**: both sides must have the same `API_KEY`. http2smtp checks `Authorization: Bearer <key>`.
- Use **HTTPS** for `TARGET_URL` (e.g. `https://.../send`).
- Rotate `API_KEY` if it leaks. An attacker with the key can send email through your account.

## Payload contract

POST to `http2smtp` `/send`:

```json
{
  "from": "Sender Name <sender@example.com>",
  "to": ["user@example.com"],
  "raw": "<base64 of full RFC 5322 message>"
}
```

`raw` contains the complete message (headers + body). Images embedded as SVG/data URIs in HTML templates are carried inside the HTML part of `raw`.

No attachment decoding or re-rendering is performed; the raw bytes are passed through.

## Environment variables

### smtp2http (internal)

| Var                  | Required | Default | Description                                                 |
| -------------------- | -------- | ------- | ----------------------------------------------------------- |
| TARGET_URL           | yes      | -       | https://.../send on http2smtp                               |
| API_KEY              | yes      | -       | Shared secret                                               |
| SMTP_DOMAIN          | yes      | -       | Domain for the self-signed certificate and SMTP greeting    |
| `SMTP_USERS_<service>` | yes    | -       | One allowed LOGIN credential per client, as `username:password`. Replace `<service>` with any label, e.g. `SMTP_USERS_BILLING=billing:secret` |
| PORT                 | no       | 8080    | HTTP health listener                                        |
| SMTP_TLS_DNS_NAMES   | no       | -       | Comma-separated extra DNS names for the self-signed certificate, added to `localhost`, `smtp2http`, `*.internal` and `SMTP_DOMAIN` |

### http2smtp (external)

| Var             | Required | Description                                |
| --------------- | -------- | ------------------------------------------ |
| API_KEY         | yes      | Same as above                              |
| SMTP_HOST       | yes      | Outbound SMTP host                         |
| SMTP_PORT       | yes      | 587 (STARTTLS) typical                     |
| SMTP_USER       | yes      | SMTP username                              |
| SMTP_PASS       | yes      | SMTP password                              |
| SMTP_FROM       | yes      | Fallback envelope from if message has none |
| SMTP_INSECURE   | yes      | "true" to disable TLS (plain 25)           |
| PORT            | yes      | HTTP listen                                |
| HEALTHCHECK_URL | no       | healthchecks.io URL to ping every hour     |

## Health endpoints

Both services expose:

- `GET /health` → `200 OK "ok"`

For http2smtp (external only): if `HEALTHCHECK_URL` is set, a GET is issued to it every hour (best-effort, errors logged).

## Deployment

### smtp2http (internal)

SMTP listens on port 587 and the health endpoint on `$PORT`. See [`smtp2http/.env.example`](smtp2http/.env.example) for the variables.

```bash
docker run -d --name smtp2http \
  --env-file smtp2http/.env \
  -p 127.0.0.1:587:587 \
  ghcr.io/pecatatoshev/smtp-over-http/smtp2http:latest
```

Keep it on a private network. Do **not** expose it publicly unless you want to accept SMTP from outside.

#### Adding or rotating an SMTP password

Each client gets its own `SMTP_USERS_<service>` variable, with the value `username:password`. The suffix after `SMTP_USERS_` is only a label; clients log in with the username from the value.

1. Generate a password:

   ```bash
   openssl rand -hex 32
   ```

2. Add a line to `smtp2http/.env`, using a new `<service>` suffix and the generated password:

   ```
   SMTP_USERS_BILLING=billing:<generated-password>
   ```

3. Recreate the container. Credentials are read only at startup, and `docker restart` does not reload `--env-file`:

   ```bash
   docker rm -f smtp2http
   docker run -d --name smtp2http \
     --env-file smtp2http/.env \
     -p 127.0.0.1:587:587 \
     ghcr.io/pecatatoshev/smtp-over-http/smtp2http:latest
   ```

4. Confirm the username appears in `allowed_users` in the startup log:

   ```bash
   docker logs smtp2http 2>&1 | grep allowed_users
   ```

The username cannot contain `:`; the password can.

To rotate a password without downtime for the client, add a second variable with the same username and the new password, recreate the container, switch the client to the new password, then remove the old variable and recreate the container again.

### http2smtp (external)

See [`http2smtp/.env.example`](http2smtp/.env.example) for the variables.

```bash
docker run -d --name http2smtp \
  --env-file http2smtp/.env \
  -p 8080:8080 \
  ghcr.io/pecatatoshev/smtp-over-http/http2smtp:latest
```

- Put behind HTTPS (reverse proxy, Cloudflare Tunnel, Tailscale, etc.).
- Give it a stable DNS name; point `smtp2http` `TARGET_URL` at `https://<host>/send`.
- Provide `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM` and `SMTP_INSECURE` for your real mail server.

### Building locally

The build context must be the repository root, because both services share one `go.mod`:

```bash
docker build -f http2smtp/Dockerfile -t http2smtp .
docker build -f smtp2http/Dockerfile -t smtp2http .
```

## Running locally (dev)

```bash
# From the repository root

# Terminal 1 - http2smtp (external side)
API_KEY=dev \
SMTP_HOST=mail.example.com \
SMTP_PORT=587 \
SMTP_USER=user \
SMTP_PASS=pass \
SMTP_FROM=sender@example.com \
SMTP_INSECURE=false \
PORT=8081 \
go run ./http2smtp

# Terminal 2 - smtp2http (internal side)
TARGET_URL=http://localhost:8081/send \
API_KEY=dev \
PORT=8080 \
SMTP_DOMAIN=localhost \
SMTP_USERS_DEV=u:p \
go run ./smtp2http

# Test with swaks or similar (LOGIN auth required)
swaks --server localhost:587 --auth LOGIN --auth-user u --auth-password p \
  --from test@example.com --to dest@example.com --data ./test.eml
```

## Notes

- The raw message is forwarded as-is. DKIM signatures (if any) remain intact.
- Large messages: smtp2http caps at 10 MiB (`MaxMessageBytes`).
- No retries: if http2smtp is unreachable or returns non-2xx, the SMTP transaction fails for the client.
- Only HTML bodies with possible inline SVG/data images are expected; no special CID/attachment unpacking is implemented.
