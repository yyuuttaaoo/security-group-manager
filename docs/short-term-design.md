# Historical Short-Term Design: Basic Authentication & Authorization

This document describes the original interim password-based plan. The current implementation uses Alipay/Baidu OAuth identity mapping instead.

## Goal
Implement a simple, quick-to-deploy authentication and authorization system using username/password stored in the configuration file. This is an interim solution to secure the application immediately.

## Architecture

### 1. Configuration (`config.yaml`)
Add a `users` section to define access control.
```yaml
auth:
  enabled: true
  session_secret: "CHANGE_ME_RANDOM_32_BYTES_OR_MORE"
  users:
    - username: "example-user"
      password: "BASE64_ENCODED_PLACEHOLDER"
      groups: ["example-group"]
```

### 2. Backend Changes (`cmd/server/main.go` & `pkg/auth`)
*   **Session Management**: Use a simple signed cookie (e.g., using `gorilla/sessions` or a simple HMAC implementation) to store the logged-in username.
*   **New Endpoints**:
    *   `POST /api/login`: Accepts `{username, password}`. Validates against config. Sets session cookie.
    *   `POST /api/logout`: Clears session cookie.
    *   `GET /api/user/info`: Returns current user info and allowed groups.
*   **Middleware/Interceptor**:
    *   Wrap `/api/update` to check for a valid session.
    *   Verify if the user is authorized for the requested `group`.

### 3. Frontend Changes (`web/index.html`)
*   **State Management**:
    *   On load, call `/api/user/info`.
    *   If 401/403, show **Login View**.
    *   If 200, show **Update View**.
*   **Login View**:
    *   Simple form: Username, Password, Login Button.
*   **Update View**:
    *   Replace the "Group Name" text input with a **Dropdown (<select>)**.
    *   Populate options from the user's allowed groups.
    *   Add a "Logout" button.

## Implementation Plan

### Phase 1: Backend Core
1.  Modify `Config` struct to include `Auth` and `Users`.
2.  Implement `AuthMiddleware` to verify cookies.
3.  Implement `LoginHandler` to validate credentials.
4.  Update `handleUpdate` to check `AuthMiddleware` context and verify group permission.

### Phase 2: Frontend
1.  Create `Login` UI component (hidden by default).
2.  Create `Main` UI component (hidden by default).
3.  Implement logic to switch views based on `/api/user/info` response.
4.  Dynamic group selector.

## Security Considerations
*   **Passwords**: Stored as Base64 (Weak, but meets "short-term" requirement). **WARNING**: Base64 is encoding, not encryption.
*   **Transport**: Should run behind HTTPS in production.
*   **Session**: HttpOnly, Secure cookies.
