package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io" // Kept because io.MultiWriter is used
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/yyuuttaaoo/security-group-manager/pkg/auth"
	"github.com/yyuuttaaoo/security-group-manager/pkg/config"
	"github.com/yyuuttaaoo/security-group-manager/pkg/logger"
	"github.com/yyuuttaaoo/security-group-manager/pkg/manager"
	"github.com/yyuuttaaoo/security-group-manager/pkg/utils"
)

var authenticator *auth.Authenticator

type UpdateRequest struct {
	IP    string `json:"ip"`
	Group string `json:"group"`
}

type UpdateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Logs    string `json:"logs"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type UserInfoResponse struct {
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := authenticator.Login(req.Username, req.Password)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	authenticator.SetSession(w, user)
	json.NewEncoder(w).Encode(LoginResponse{Success: true, Message: "Logged in"})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	authenticator.ClearSession(w)
	json.NewEncoder(w).Encode(LoginResponse{Success: true, Message: "Logged out"})
}

func handleUserInfo(w http.ResponseWriter, r *http.Request) {
	user, err := authenticator.GetUserFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	json.NewEncoder(w).Encode(UserInfoResponse{
		Username: user.Username,
		Groups:   user.Groups,
	})
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check permission
	user := r.Context().Value(auth.UserContextKey).(*auth.User)

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.IP == "" {
		http.Error(w, "IP is required", http.StatusBadRequest)
		return
	}

	if err := utils.ValidateIP(req.IP); err != nil {
		http.Error(w, fmt.Sprintf("Invalid IP: %v", err), http.StatusBadRequest)
		return
	}

	groupName := req.Group
	if groupName == "" {
		groupName = "default"
	}

	// Verify group access
	if !authenticator.HasGroupAccess(user, groupName) {
		http.Error(w, fmt.Sprintf("You do not have permission to modify group '%s'", groupName), http.StatusForbidden)
		return
	}

	slog.Info("Received update request", "ip", req.IP, "group", groupName, "user", user.Username)

	var logBuffer bytes.Buffer
	regions := []string{"cn-hongkong", "ap-northeast-1", "us-west-1"}

	// Create a multi-writer to write to both the configured log writer and the buffer
	// Use logger.LogWriter to ensure it goes to file if configured
	mw := io.MultiWriter(logger.LogWriter, &logBuffer)

	// Create a logger that writes to the multi-writer
	// We use a TextHandler for the report output to be readable
	reqLogger := slog.New(slog.NewTextHandler(mw, nil))

	var errOccurred bool
	for _, region := range regions {
		reqLogger.Info("Processing Region", "region", region, "group", groupName)

		err := manager.ProcessRegion(region, req.IP, groupName, reqLogger)
		if err != nil {
			reqLogger.Error("Error processing region", "region", region, "error", err)
			errOccurred = true
		}
	}

	resp := UpdateResponse{
		Success: !errOccurred,
		Message: "Update process completed",
		Logs:    logBuffer.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type GeoInfo struct {
	IP          string `json:"ip"`
	City        string `json:"city"`
	Region      string `json:"region"`
	CountryName string `json:"country_name"`
	Org         string `json:"org"`
}

func handleGetIP(w http.ResponseWriter, r *http.Request) {
	// Get IP from RemoteAddr
	ip := r.RemoteAddr
	// Handle X-Forwarded-For if behind proxy (optional, but good for real usage)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = forwarded
	}

	// Strip port if present
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	} else {
		// If SplitHostPort fails (e.g. no port), use original
		// But RemoteAddr usually has port. X-Forwarded-For might be a list.
		if strings.Contains(ip, ",") {
			ip = strings.TrimSpace(strings.Split(ip, ",")[0])
		}
	}

	// Fetch Geo Info for this IP
	respData := GeoInfo{IP: ip}

	// Only fetch if it's a valid public IP (simple check)
	// Skip for localhost/private IPs to avoid errors or useless requests
	if ip != "127.0.0.1" && ip != "::1" && !strings.HasPrefix(ip, "192.168.") && !strings.HasPrefix(ip, "10.") {
		client := &http.Client{}
		req, err := http.NewRequest("GET", fmt.Sprintf("https://ipapi.co/%s/json/", ip), nil)
		if err == nil {
			req.Header.Set("User-Agent", "SecurityGroupManager/1.0")
			resp, err := client.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var geo GeoInfo
					if err := json.NewDecoder(resp.Body).Decode(&geo); err == nil {
						// Use the IP we detected, but fill in other info
						respData.City = geo.City
						respData.Region = geo.Region
						respData.CountryName = geo.CountryName
						respData.Org = geo.Org
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(respData)
}

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		// If config file error (other than not found), log it but proceed with defaults?
		// Or just panic? Let's print to stderr and proceed with defaults if LoadConfig returns defaults on error
		// But LoadConfig returns nil on parse error.
		if cfg == nil {
			fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
			os.Exit(1)
		}
	}

	// Setup logger
	logger.Setup(cfg.Log)
	slog.Info("Starting server...", "config", cfg)

	// Init Authenticator
	authenticator = auth.NewAuthenticator(cfg.Auth)

	// Serve static files from the "web" directory
	fs := http.FileServer(http.Dir("web"))
	http.Handle("/", fs)

	// Auth endpoints
	http.HandleFunc("/api/login", handleLogin) // Login cannot require CSRF token as it establishes the session
	http.HandleFunc("/api/logout", authenticator.CSRFMiddleware(handleLogout))
	http.HandleFunc("/api/user/info", handleUserInfo)

	// Protected endpoints
	http.HandleFunc("/api/update", authenticator.CSRFMiddleware(authenticator.Middleware(handleUpdate)))

	// Public endpoints
	http.HandleFunc("/api/ip", handleGetIP)

	port := os.Getenv("PORT")
	if port == "" {
		if cfg.Server.Port != "" {
			port = cfg.Server.Port
		} else {
			port = "8080"
		}
	}

	addr := cfg.Server.Address + ":" + port
	slog.Info("Server listening", "address", addr, "tls", cfg.Server.TLS)

	if cfg.Server.TLS {
		if err := http.ListenAndServeTLS(addr, cfg.Server.CertFile, cfg.Server.KeyFile, nil); err != nil {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	} else {
		if err := http.ListenAndServe(addr, nil); err != nil {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}
}
