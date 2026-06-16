# Deployment Guide: Security Group Manager

This guide details how to deploy the Security Group Manager to your server and configure Caddy as a reverse proxy with automatic HTTPS.

Replace the following placeholders throughout this guide:
- `<YOUR_SERVER_IP>` — your server's public IP address
- `<YOUR_DOMAIN>` — your domain or subdomain (e.g. `example.com`)
- `<YOUR_USER>` — your SSH user (e.g. `root` or `admin`)

## Prerequisites

- Local machine with Go installed.
- SSH access to `<YOUR_USER>@<YOUR_SERVER_IP>`.
- Domain with DNS management access.

## 1. Build the Application (Local)

Compile the application for Linux locally to avoid server resource exhaustion.

```bash
# -ldflags="-s -w" strips debug symbols, reducing binary size by ~30%
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/server-linux cmd/server/main.go
```

## 2. Create a Dedicated System User (Server)

Run the service under a dedicated system user with minimal permissions.

```bash
ssh <YOUR_USER>@<YOUR_SERVER_IP>
# Create a system user 'web' without a login shell
sudo adduser --system --group --no-create-home --shell /bin/false web
exit
```

## 3. Prepare the Server

```bash
ssh <YOUR_USER>@<YOUR_SERVER_IP>
sudo mkdir -p /opt/security-group-manager/web
sudo chown -R web:web /opt/security-group-manager
exit
```

## 4. Transfer Files

Use `-C` for SSH compression (significantly faster on slow links). Stage files via `/tmp` and move in one SSH session.

```bash
# Upload
scp -C bin/server-linux <YOUR_USER>@<YOUR_SERVER_IP>:/tmp/server
scp -C web/index.html <YOUR_USER>@<YOUR_SERVER_IP>:/tmp/index.html
# First deploy only: create a private config from config.yaml.template, fill real secrets locally,
# then upload that private config. Do not commit it.
scp -C config.yaml <YOUR_USER>@<YOUR_SERVER_IP>:/tmp/config.yaml

# Move into place and fix permissions in one SSH session:
ssh <YOUR_USER>@<YOUR_SERVER_IP> '
  mv /tmp/server /opt/security-group-manager/server
  chmod +x /opt/security-group-manager/server
  mv /tmp/index.html /opt/security-group-manager/web/index.html
  # First deploy only:
  # mv /tmp/config.yaml /opt/security-group-manager/config.yaml
  # chmod 600 /opt/security-group-manager/config.yaml
  chown -R web:web /opt/security-group-manager
'
```

## 5. Configure DNS

Add an **A Record** in your DNS provider:
- **Name**: `@` (root) or a subdomain (e.g. `sgm`)
- **Value**: `<YOUR_SERVER_IP>`
- **TTL**: Default (1 Hour)

## 6. Install and Configure Caddy (Server)

### Install Caddy (Debian/Ubuntu)

```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy
```

### Configure Caddy

```bash
sudo vim /etc/caddy/Caddyfile
```

```caddyfile
<YOUR_DOMAIN> {
    reverse_proxy localhost:8080
}
```

```bash
sudo systemctl restart caddy
```

## 7. Run the Service (Systemd)

```bash
sudo vim /etc/systemd/system/web.service
```

```ini
[Unit]
Description=Security Group Manager
After=network.target

[Service]
Type=simple
User=web
Group=web
WorkingDirectory=/opt/security-group-manager
ExecStart=/opt/security-group-manager/server
Restart=on-failure
Environment=PORT=8080

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable web
sudo systemctl start web
sudo systemctl status web
```

## 8. Quick Redeploy (routine updates)

For day-to-day code changes (binary + web assets only):

```bash
# 1. Build
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/server-linux cmd/server/main.go

# 2. Upload
scp -C bin/server-linux <YOUR_USER>@<YOUR_SERVER_IP>:/tmp/server
scp -C web/index.html <YOUR_USER>@<YOUR_SERVER_IP>:/tmp/index.html

# 3. Deploy & restart
ssh <YOUR_USER>@<YOUR_SERVER_IP> '
  mv /tmp/server /opt/security-group-manager/server && \
  chmod +x /opt/security-group-manager/server && \
  mv /tmp/index.html /opt/security-group-manager/web/index.html && \
  chown -R web:web /opt/security-group-manager && \
  systemctl restart web && \
  systemctl is-active web
'
```

## 9. Maintenance & Changes

### Changing the Domain

1. **DNS**: Update the A record to point to the new IP or add a new subdomain record.
2. **Caddy**: Edit `/etc/caddy/Caddyfile` and replace the domain.
3. **Restart Caddy**: `sudo systemctl restart caddy`.

### Changing the Port

If you change the app port in `config.yaml` (e.g., to `9090`):

1. **App Config**: Update `config.yaml` on the server:
    ```yaml
    server:
      port: "9090"
    ```
2. **Systemd**: Update `/etc/systemd/system/web.service`:
    ```ini
    Environment=PORT=9090
    ```
    Then: `sudo systemctl daemon-reload && sudo systemctl restart web`.
3. **Caddy**: Update `/etc/caddy/Caddyfile` with the new port, then `sudo systemctl restart caddy`.
