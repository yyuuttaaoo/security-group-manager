# Security Group Manager - Agent Guide

## Project Overview
**Security Group Manager** is a tool designed to automatically update Alibaba Cloud Security Group and SWAS (Simple Application Server) Firewall rules. Its primary purpose is to allow the current IP address of the user or server to access specific resources, managing these rules dynamically based on tags and description prefixes.

## Key Features
- **Automated Rule Updates**: Automatically updates `SourceCidrIp` for security group rules and firewall rules.
- **Multi-Region Support**: Currently configured for `cn-hongkong`, `ap-northeast-1`, and `us-west-1`.
- **Dynamic Management**:
  - Targets Security Groups/Instances with tag `auto-manage: true`.
  - Targets Rules with description/remark prefix `auto-manage-{group}-`.
- **Authentication & Authorization**:
  - **Login System**: Alipay and Baidu OAuth login with provider IDs mapped in config.
  - **ACL**: Users are restricted to specific groups (e.g., `shenjin`, `iphone17pro`).
  - **Session Management**: Secure cookie-based sessions with HMAC signing.
- **Security**:
  - **CSRF Protection**: Double Submit Cookie pattern for all state-changing requests.
  - **HTTPS Support**: Built-in TLS support or reverse proxy ready.
  - **Secure Defaults**: Configurable `Secure` cookie flag.
- **Modes**:
  - **Server Mode**: HTTP API with Web UI for login and rule updates.
  - **Demo Mode**: CLI tool to update rules based on the machine's current public IP.
- **Logging**: Configurable logging to stdout or file with rotation (lumberjack).

## Architecture

### Directory Structure
- `cmd/server`: Entry point for the HTTP server application.
- `cmd/demo`: Entry point for the CLI demo application.
- `pkg/manager`: Core business logic for interacting with Alibaba Cloud APIs.
- `pkg/auth`: Authentication, session management, and CSRF protection logic.
- `pkg/config`: Configuration loading logic.
- `pkg/logger`: Logging configuration and setup.
- `pkg/utils`: Utility functions (IP validation, getting current IP).
- `pkg/alipay`: Native Alipay OAuth client, signing, and certificate helpers.
- `pkg/baidu`: Baidu OAuth client.
- `web`: Static frontend files (HTML/JS/CSS).

### Core Components
- **SecurityGroupManager** (`pkg/manager/manager.go`): Wraps the Alibaba Cloud ECS SDK to manage Security Groups.
- **SWASManager** (`pkg/manager/manager.go`): Wraps the Alibaba Cloud SWAS SDK to manage Lightweight Application Server firewalls.
- **Authenticator** (`pkg/auth/auth.go`): Handles provider identity mapping, session cookie management (HMAC signed), and CSRF token generation/validation.
- **ProcessRegion** (`pkg/manager/manager.go`): The main orchestration function. It iterates through configured regions, finds resources with the `auto-manage` tag, and updates rules matching the `auto-manage-{group}-` prefix.

## Configuration
The application uses `config.yaml` for configuration.

```yaml
log:
  output: "file"
  file_path: "server.log"
  max_size: 10 # MB
  max_backups: 3
  max_age: 28 # days
  compress: true

auth:
  enabled: true
  session_secret: "YOUR_RANDOM_SECRET" # Must be strong
  cookie_secure: true # Set to false for local HTTP testing
  users:
    - uid: "admin"
      alipay_userid: "YOUR_ALIPAY_USER_ID"
      baidu_openid: "YOUR_BAIDU_OPENID"
      groups: ["default", "admin"]

alipay:
  app_id: "YOUR_ALIPAY_APP_ID"
  private_key_path: "cert/app_private_key.txt"
  app_cert_path: "cert/app_cert.crt"
  alipay_public_cert_path: "cert/alipay_cert.crt"
  alipay_root_cert_path: "cert/alipay_root_cert.crt"
  redirect_uri: "https://your-domain.example/api/oauth/alipay/callback"

baidu:
  app_id: "YOUR_BAIDU_APP_ID"
  app_key: "YOUR_BAIDU_APP_KEY"
  secret_key: "YOUR_BAIDU_SECRET_KEY"
  sign_key: "YOUR_BAIDU_SIGN_KEY"
  redirect_uri: "https://your-domain.example/api/oauth/baidu/callback"

server:
  address: "127.0.0.1" # Listen address (e.g., 0.0.0.0 for public)
  port: "8080"
  tls: false # Enable built-in HTTPS
  cert_file: "server.crt"
  key_file: "server.key"
```

## API Endpoints

### Public
- `GET /api/ip`: Returns the caller's public IP and Geo info.

### Authenticated
- `GET /api/oauth/alipay/login`: Starts Alipay OAuth login.
- `GET|POST /api/oauth/alipay/callback`: Completes Alipay OAuth login.
- `GET /api/oauth/baidu/login`: Starts Baidu OAuth login.
- `GET /api/oauth/baidu/callback`: Completes Baidu OAuth login.
- `POST /api/logout`: Logs out a user (Requires CSRF).
- `GET /api/user/info`: Returns current user info and allowed groups.
- `POST /api/update`: Triggers rule update for a specific group (Requires CSRF + Auth).

## Development Guidelines

### Prerequisites
- Go 1.25+
- Alibaba Cloud Credentials configured.

### Running the Server
```bash
go run cmd/server/main.go
```

### Running the Demo CLI
```bash
go run cmd/demo/main.go -group <group_name>
```

### Building
```bash
go build -o bin/server cmd/server/main.go
go build -o bin/demo cmd/demo/main.go
```

## Deployment
See `deploy.md` for detailed deployment instructions, including Caddy reverse proxy setup.

## Future Improvements
- **Database**: Move user/group config to a database for dynamic management.
- **Concurrency**: Process regions in parallel.
