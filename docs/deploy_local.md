# Deployment Guide: Security Group Manager

This guide details how to deploy the Security Group Manager to your server (`<YOUR_SERVER_IP>`) and configure Caddy as a reverse proxy with automatic HTTPS for `<YOUR_DOMAIN>`.

## Prerequisites

- Local machine with Go installed.
- SSH access to `root@<YOUR_SERVER_IP>`.
- Access to GoDaddy DNS management for `<YOUR_DOMAIN>`.

## 1. Build the Application (Local)

First, compile the application for Linux (since your server is likely Linux).

```bash
# In your local project root (recommended: build locally to avoid server resource exhaustion)
# -ldflags="-s -w" strips debug symbols, reducing binary size by ~30%
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/server-linux cmd/server/main.go
```

## 2. Create a Dedicated System User (Server)

For security, it is highly recommended to run the service under a dedicated system user with minimal permissions.

```bash
ssh root@<YOUR_SERVER_IP>
# Create a system user 'web' without a login shell
sudo adduser --system --group --no-create-home --shell /bin/false web
exit
```

## 3. Prepare the Server

SSH into your server and create the service directory.

```bash
ssh root@<YOUR_SERVER_IP>
# On server:
sudo mkdir -p /opt/security-group-manager
sudo mkdir -p /opt/security-group-manager/web
# Change ownership to the 'web' user
sudo chown -R web:web /opt/security-group-manager
exit
```

## 4. Transfer Files

Copy the binary, config, and web assets to the server.

```bash
# In your local project root
# -C enables SSH compression (significantly faster on slow links)
scp -C bin/server-linux root@<YOUR_SERVER_IP>:/tmp/server
scp -C web/index.html root@<YOUR_SERVER_IP>:/tmp/index.html
# Only needed on first deploy or if config changed. Create this private file from
# config.yaml.template and fill real secrets locally. Do not commit it.
scp -C config.yaml root@<YOUR_SERVER_IP>:/tmp/config.yaml

# Move into place and fix permissions in one SSH session:
ssh root@<YOUR_SERVER_IP> '
  mv /tmp/server /opt/security-group-manager/server
  chmod +x /opt/security-group-manager/server
  mv /tmp/index.html /opt/security-group-manager/web/index.html
  # First deploy only:
  # mv /tmp/config.yaml /opt/security-group-manager/config.yaml
  # chmod 600 /opt/security-group-manager/config.yaml
  chown -R web:web /opt/security-group-manager
'
```

## 5. Configure DNS (GoDaddy)

1. Log in to your GoDaddy account.
2. Navigate to **DNS Management** for `<YOUR_DOMAIN>`.
3. Add an **A Record**:
    - **Name**: `@` (root) or `web` (subdomain, e.g., web.<YOUR_DOMAIN>)
    - **Value**: `<YOUR_SERVER_IP>`
    - **TTL**: Default (e.g., 1 Hour)
4. Save the changes. It may take a few minutes to propagate.

## 6. Install and Configure Caddy (Server)

SSH back into your server.

### Install Caddy (Debian/Ubuntu)

```bash
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy
```

### Configure Caddy

Create or edit `/etc/caddy/Caddyfile`:

```bash
sudo vim /etc/caddy/Caddyfile
```

Add the following content (replace `<YOUR_DOMAIN>` with your actual domain/subdomain):

```caddyfile
<YOUR_DOMAIN> {
    reverse_proxy localhost:8080
}
```

Restart Caddy:

```bash
sudo systemctl restart caddy
```

## 7. Run the Service (Systemd)

Create a systemd service to keep the app running.

```bash
sudo vim /etc/systemd/system/web.service
```

Content:

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

Enable and start the service:

```bash
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
scp -C bin/server-linux root@<YOUR_SERVER_IP>:/tmp/server
scp -C web/index.html root@<YOUR_SERVER_IP>:/tmp/index.html

# 3. Deploy & restart
ssh root@<YOUR_SERVER_IP> '
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

1. **DNS**: Update the A record in GoDaddy to point to the new IP (if IP changed) or add a new record for a new subdomain.
2. **Caddy**: Edit `/etc/caddy/Caddyfile`:

    ```diff
    - <YOUR_DOMAIN> {
    + new-domain.com {
        reverse_proxy localhost:8080
    }
    ```

3. **Restart Caddy**: `sudo systemctl restart caddy`.

### Changing the Port

If you change the app port in `config.yaml` (e.g., to `9090`):

1. **App Config**: Update `config.yaml` on the server:

    ```yaml
    # Edit /opt/security-group-manager/config.yaml
    server:
      port: "9090"
    ```

2. **Systemd**: If you set PORT via env var in systemd, update `/etc/systemd/system/sgm.service`:

    ```ini
    Environment=PORT=9090
    ```

    Then run `sudo systemctl daemon-reload` and `sudo systemctl restart web`.
3. **Caddy**: Update `/etc/caddy/Caddyfile`:

    ```caddyfile
    <YOUR_DOMAIN> {
        reverse_proxy localhost:9090
    }
    ```

    Then `sudo systemctl restart caddy`.
