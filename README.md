# tesla-sentry

Automatically toggles Tesla Sentry Mode on a schedule using the Tesla Fleet API.

## Prerequisites

- **Go 1.23+** — `go version` must show `go1.23` or later
- **Tesla account** with a vehicle on your account
- **Cloudflare Pages site** (free tier) — used to host the public key at a stable `https://xxx.pages.dev` URL

## Build

```bash
go build -o tesla-sentry ./cmd/tesla-sentry
sudo install tesla-sentry /usr/local/bin/
```

## One-time Setup

Work through these steps in order. Each step depends on the previous one.

### 1. Generate the key pair

```bash
tesla-sentry keygen
```

This writes two files to `~/.config/tesla-sentry/`:

| File | Permissions | Purpose |
|---|---|---|
| `private-key.pem` | 0600 | Signs vehicle commands |
| `public-key.pem` | 0644 | Hosted publicly so Tesla can verify your commands |

It also prints the required hosting path:

```
Host the PUBLIC key at: https://<domain>/.well-known/appspecific/com.tesla.3p.public-key.pem
```

### 2. Deploy the public key to Cloudflare Pages

Upload `~/.config/tesla-sentry/public-key.pem` to your Cloudflare Pages project at this exact path:

```
/.well-known/appspecific/com.tesla.3p.public-key.pem
```

Note your Pages domain — it looks like `xxx.pages.dev`. You will use it throughout the remaining steps.

Verify the file is accessible:

```bash
curl https://xxx.pages.dev/.well-known/appspecific/com.tesla.3p.public-key.pem
```

### 3. Create the Tesla developer app

Go to [developer.tesla.com](https://developer.tesla.com) and create an application with these settings:

| Setting | Value |
|---|---|
| Allowed Origin | `https://xxx.pages.dev` |
| Redirect URI | `https://xxx.pages.dev/callback` |
| Scopes | `vehicle_device_data vehicle_cmds` |

After creating the app, copy the **Client ID** and **Client Secret**.

### 4. Write config.json

```bash
cat > ~/.config/tesla-sentry/config.json << 'EOF'
{
  "client_id":     "YOUR_CLIENT_ID",
  "client_secret": "YOUR_CLIENT_SECRET",
  "vin":           "YOUR_17_CHAR_VIN",
  "domain":        "xxx.pages.dev",
  "region":        "na"
}
EOF
chmod 600 ~/.config/tesla-sentry/config.json
```

> `region` must be `na` for North America. The Fleet API base URL is
> `https://fleet-api.prd.na.vn.cloud.tesla.com`.

Config directory honors `XDG_CONFIG_HOME`; defaults to `~/.config/tesla-sentry/`.

### 5. Register your domain with Tesla

```bash
tesla-sentry register
```

This obtains a partner token and calls the Fleet API to register your `xxx.pages.dev` domain. Tesla uses this to look up your public key.

### 6. Log in (user OAuth)

```bash
tesla-sentry login
```

The command prints an authorization URL:

```
1. Open this URL in a browser and approve:
   https://auth.tesla.com/oauth2/v3/authorize?...
2. After redirect, copy the `code` query parameter from the URL bar.
Paste code:
```

Open the URL, approve the requested permissions, and when Tesla redirects to `https://xxx.pages.dev/callback?code=...`, copy the `code` value from the URL bar and paste it at the prompt.

Tokens are saved to `~/.config/tesla-sentry/token.json` (0600).

### 7. Add the virtual key in the Tesla app

On your phone, open:

```
https://tesla.com/_ak/xxx.pages.dev
```

The Tesla app will prompt you to add a virtual key for this application. Accept. Without this step, signed vehicle commands will be rejected.

### 8. Verify the setup

```bash
tesla-sentry status
```

Expected output:

```
sentry mode: false
```

(or `true` if sentry mode is already on). Any error at this point points to a missing step above.

## Crontab

Enable sentry mode every night at 22:00 and disable it every morning at 07:00:

```bash
crontab -e
```

Add these two lines:

```cron
0 22 * * *  /usr/local/bin/tesla-sentry on  >> ~/.config/tesla-sentry/sentry.log 2>&1
0 7  * * *  /usr/local/bin/tesla-sentry off >> ~/.config/tesla-sentry/sentry.log 2>&1
```

Times are in the local timezone of the machine running cron.

## Troubleshooting

**Token expired** — `tesla-sentry` refreshes access tokens automatically, but if the refresh token itself expires (typically after 90 days of inactivity) you will see an authentication error. Fix: re-run `tesla-sentry login`.

**`vehicle offline`** — The command sends a wake signal and retries until the vehicle comes online. This can take up to the 3-minute command timeout. No action needed; the car will usually respond within that window.

**Refresh token rotation** — Tesla rotates the refresh token on each use. The new token is written back to `token.json` automatically. Never copy or restore an old `token.json`; it will invalidate the current session.
