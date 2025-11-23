# Long-Term Design: OAuth2 Integration

## Goal
Transition from local password storage to a secure, third-party authentication system (e.g., Alipay, GitHub, Google). This eliminates the need to store sensitive credentials locally and leverages established identity providers.

## Architecture

### 1. Configuration (`config.yaml`)
Replace `users` password fields with an ACL map based on Provider User IDs.
```yaml
auth:
  provider: "alipay" # or "github", "google"
  client_id: "your-client-id"
  client_secret: "your-client-secret"
  callback_url: "https://your-domain.com/auth/callback"
  session_secret: "..."
  acl:
    - user_id: "208812345678" # Alipay User ID
      username: "yutao" # Display name
      groups: ["shenjin", "iphone17pro"]
```

### 2. Backend Changes
*   **OAuth Flow**:
    *   `GET /auth/login`: Redirects user to the Provider's authorization page.
    *   `GET /auth/callback`:
        1.  Receives `code`.
        2.  Exchanges `code` for `access_token`.
        3.  Fetches User Info (specifically the unique User ID).
        4.  Checks `acl` in config. If User ID exists, create session. If not, deny access.
*   **Session**: Same session mechanism as Short-Term, but the session creation source changes.

### 3. Frontend Changes
*   **Login Page**:
    *   Replace "Username/Password" form with a "Login with Alipay" (or other provider) button.
    *   Button links to `/auth/login`.

## Feasibility Analysis
**Is Third-Party Auth Only Feasible?**
**Yes.** This is a standard pattern for internal tools and modern web apps.
*   **Pros**:
    *   **Security**: No passwords stored locally. No risk of weak passwords.
    *   **UX**: One-click login.
    *   **Maintenance**: Offload identity management to the provider.
*   **Cons**:
    *   Requires the app to be accessible via a domain (for callbacks) or careful local configuration.
    *   Dependency on external service.

## Migration Plan
1.  **Refactor Auth Interface**: Ensure the backend `Auth` logic is an interface (e.g., `Authenticator`).
2.  **Implement OAuth Provider**: Create an implementation for the chosen provider (e.g., `AlipayAuthenticator`).
3.  **Update Config**: Switch config schema to support provider settings.
4.  **Update Frontend**: Swap the login form for the OAuth button.
