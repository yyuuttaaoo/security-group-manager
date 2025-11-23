# Deployment Guide: Security Group Manager

This guide details how to deploy the Security Group Manager to your server (`47.243.41.178`) and configure Caddy as a reverse proxy with automatic HTTPS for `opentelemetry.xyz`.

## Prerequisites
- Local machine with Go installed.
- SSH access to `admin@47.243.41.178`.
- Access to GoDaddy DNS management for `opentelemetry.xyz`.

## 1. Build the Application (Local)
First, compile the application for Linux (since your server is likely Linux).

```bash
# In your local project root
GOOS=linux GOARCH=amd64 go build -o bin/server-linux cmd/server/main.go
```

## 2. Prepare the Server
SSH into your server and create the service directory.

```bash
ssh admin@47.243.41.178
# On server:
mkdir -p /home/admin/security-group-manager
mkdir -p /home/admin/security-group-manager/web
exit
```

## 3. Transfer Files
Copy the binary, config, and web assets to the server.

```bash
# In your local project root
scp bin/server-linux admin@47.243.41.178:/home/admin/security-group-manager/server
scp config.yaml admin@47.243.41.178:/home/admin/security-group-manager/config.yaml
scp -r web/* admin@47.243.41.178:/home/admin/security-group-manager/web/
```

## 4. Configure DNS (GoDaddy)
1.  Log in to your GoDaddy account.
2.  Navigate to **DNS Management** for `opentelemetry.xyz`.
3.  Add an **A Record**:
    *   **Name**: `@` (root) or `sgm` (subdomain, e.g., sgm.opentelemetry.xyz)
    *   **Value**: `47.243.41.178`
    *   **TTL**: Default (e.g., 1 Hour)
4.  Save the changes. It may take a few minutes to propagate.

## 5. Install and Configure Caddy (Server)
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

Add the following content (replace `opentelemetry.xyz` with your actual domain/subdomain):

```caddyfile
opentelemetry.xyz {
    reverse_proxy localhost:8080
}
```

Restart Caddy:
```bash
sudo systemctl restart caddy
```

## 6. Run the Service (Systemd)
Create a systemd service to keep the app running.

```bash
sudo vim /etc/systemd/system/sgm.service
```

Content:
```ini
[Unit]
Description=Security Group Manager
After=network.target

[Service]
Type=simple
User=admin
WorkingDirectory=/home/admin/security-group-manager
ExecStart=/home/admin/security-group-manager/server
Restart=on-failure
Environment=PORT=8080

[Install]
WantedBy=multi-user.target
```

Enable and start the service:
```bash
sudo systemctl enable sgm
sudo systemctl start sgm
sudo systemctl status sgm
```

## 7. Maintenance & Changes

### Changing the Domain
1.  **DNS**: Update the A record in GoDaddy to point to the new IP (if IP changed) or add a new record for a new subdomain.
2.  **Caddy**: Edit `/etc/caddy/Caddyfile`:
    ```diff
    - opentelemetry.xyz {
    + new-domain.com {
        reverse_proxy localhost:8080
    }
    ```
3.  **Restart Caddy**: `sudo systemctl restart caddy`.

### Changing the Port
If you change the app port in `config.yaml` (e.g., to `9090`):
1.  **App Config**: Update `config.yaml` on the server:
    ```yaml
    server:
      port: "9090"
    ```
2.  **Systemd**: If you set PORT via env var in systemd, update `/etc/systemd/system/sgm.service`:
    ```ini
    Environment=PORT=9090
    ```
    Then run `sudo systemctl daemon-reload` and `sudo systemctl restart sgm`.
3.  **Caddy**: Update `/etc/caddy/Caddyfile`:
    ```caddyfile
    opentelemetry.xyz {
        reverse_proxy localhost:9090
    }
    ```
    Then `sudo systemctl restart caddy`.
