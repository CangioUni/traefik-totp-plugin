// Package traefik_totp_auth implements a Traefik middleware plugin for TOTP authentication
package traefik_totp_plugin

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Config holds the plugin configuration
type Config struct {
	SecretKey       string `json:"secretKey,omitempty"`       // Base32 encoded TOTP secret
	SessionExpiry   int    `json:"sessionExpiry,omitempty"`   // Session expiry in seconds (default: 3600)
	CookieName      string `json:"cookieName,omitempty"`      // Name of the session cookie
	CookieDomain    string `json:"cookieDomain,omitempty"`    // Cookie domain
	CookieSecure    bool   `json:"cookieSecure,omitempty"`    // Use secure cookies
	Issuer          string `json:"issuer,omitempty"`          // TOTP issuer name
	AccountName     string `json:"accountName,omitempty"`     // TOTP account name
	TimeStep        int    `json:"timeStep,omitempty"`        // Time step in seconds (default: 30)
	CodeDigits      int    `json:"codeDigits,omitempty"`      // Number of digits in code (default: 6)
	AllowedSkew     int    `json:"allowedSkew,omitempty"`     // Number of time steps to allow for clock skew (default: 1)
	PageTitle       string   `json:"pageTitle,omitempty"`       // Custom page title
	PageDescription string   `json:"pageDescription,omitempty"` // Custom page description
	ValidateIP      bool     `json:"validateIP,omitempty"`      // Validate IP address for sessions (default: false)
	TrustedProxies  []string `json:"trustedProxies,omitempty"`  // CIDR ranges of trusted proxies (e.g., ["10.0.0.0/8", "172.16.0.0/12"])
}

// CreateConfig creates the default plugin configuration
func CreateConfig() *Config {
	return &Config{
		SessionExpiry:   3600, // 1 hour
		CookieName:      "totp_session",
		CookieSecure:    true,
		TimeStep:        30,
		CodeDigits:      6,
		AllowedSkew:     1,
		PageTitle:       "TOTP Authentication Required",
		PageDescription: "Please enter your TOTP code to continue",
		ValidateIP:      false, // Disabled by default for better compatibility
	}
}

// TOTPAuth is the plugin structure
type TOTPAuth struct {
	next           http.Handler
	name           string
	config         *Config
	sessions       *sessionStore
	trustedNetworks []*net.IPNet // Parsed CIDR networks for trusted proxies
}

// Session represents an authenticated session
type Session struct {
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
	IP        string
}

// sessionStore manages active sessions
type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// New creates a new TOTPAuth plugin
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if config.SecretKey == "" {
		return nil, fmt.Errorf("secretKey is required")
	}

	// Validate secret key is valid base32
	_, err := base32.StdEncoding.DecodeString(strings.ToUpper(config.SecretKey))
	if err != nil {
		return nil, fmt.Errorf("invalid secret key (must be base32 encoded): %w", err)
	}

	if config.SessionExpiry <= 0 {
		config.SessionExpiry = 3600
	}

	if config.TimeStep <= 0 {
		config.TimeStep = 30
	}

	if config.CodeDigits <= 0 {
		config.CodeDigits = 6
	}

	if config.AllowedSkew < 0 {
		config.AllowedSkew = 1
	}

	// Parse trusted proxy CIDR ranges
	var trustedNetworks []*net.IPNet
	for _, cidr := range config.TrustedProxies {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR in trustedProxies (%s): %w", cidr, err)
		}
		trustedNetworks = append(trustedNetworks, network)
	}

	plugin := &TOTPAuth{
		next:            next,
		name:            name,
		config:          config,
		sessions:        &sessionStore{
			sessions: make(map[string]*Session),
		},
		trustedNetworks: trustedNetworks,
	}

	// Start cleanup goroutine
	go plugin.cleanupExpiredSessions(ctx)

	return plugin, nil
}

// ServeHTTP handles the HTTP request
func (ta *TOTPAuth) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Check if user has valid session
	if ta.hasValidSession(req) {
		ta.next.ServeHTTP(rw, req)
		return
	}

	// Check if this is a TOTP submission
	if req.Method == http.MethodPost && req.URL.Path == req.URL.Path {
		ta.handleTOTPSubmission(rw, req)
		return
	}

	// Show TOTP input page
	ta.showTOTPPage(rw, req, "")
}

// hasValidSession checks if the request has a valid session cookie
func (ta *TOTPAuth) hasValidSession(req *http.Request) bool {
	cookie, err := req.Cookie(ta.config.CookieName)
	if err != nil {
		return false
	}

	ta.sessions.mu.RLock()
	session, exists := ta.sessions.sessions[cookie.Value]
	ta.sessions.mu.RUnlock()

	if !exists {
		return false
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		ta.sessions.mu.Lock()
		delete(ta.sessions.sessions, cookie.Value)
		ta.sessions.mu.Unlock()
		return false
	}

	// Verify IP address if enabled (optional security check)
	if ta.config.ValidateIP {
		clientIP := ta.getClientIP(req)
		if session.IP != clientIP {
			log.Printf("[%s] Session IP mismatch: expected %s, got %s", ta.name, session.IP, clientIP)
			ta.sessions.mu.Lock()
			delete(ta.sessions.sessions, cookie.Value)
			ta.sessions.mu.Unlock()
			return false
		}
	}

	return true
}

// handleTOTPSubmission processes TOTP code submission
func (ta *TOTPAuth) handleTOTPSubmission(rw http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		ta.showTOTPPage(rw, req, "Invalid request")
		return
	}

	code := strings.TrimSpace(req.FormValue("totp_code"))
	if code == "" {
		ta.showTOTPPage(rw, req, "Please enter a TOTP code")
		return
	}

	// Validate TOTP code
	if !ta.validateTOTP(code) {
		log.Printf("[%s] Invalid TOTP code attempt from %s", ta.name, ta.getClientIP(req))
		ta.showTOTPPage(rw, req, "Invalid TOTP code. Please try again.")
		return
	}

	// Create new session
	sessionToken, err := ta.createSession(req)
	if err != nil {
		log.Printf("[%s] Failed to create session: %v", ta.name, err)
		ta.showTOTPPage(rw, req, "Authentication failed. Please try again.")
		return
	}

	// Set session cookie
	http.SetCookie(rw, &http.Cookie{
		Name:     ta.config.CookieName,
		Value:    sessionToken,
		Path:     "/",
		Domain:   ta.config.CookieDomain,
		MaxAge:   ta.config.SessionExpiry,
		Secure:   ta.config.CookieSecure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	log.Printf("[%s] Successful TOTP authentication from %s", ta.name, ta.getClientIP(req))

	// Redirect to original URL
	http.Redirect(rw, req, req.URL.String(), http.StatusSeeOther)
}

// validateTOTP validates a TOTP code
func (ta *TOTPAuth) validateTOTP(code string) bool {
	// Get current time step
	currentTimeStep := time.Now().Unix() / int64(ta.config.TimeStep)

	// Check current time step and allow for skew
	for skew := -ta.config.AllowedSkew; skew <= ta.config.AllowedSkew; skew++ {
		timeStep := currentTimeStep + int64(skew)
		expectedCode := ta.generateTOTP(timeStep)
		if code == expectedCode {
			return true
		}
	}

	return false
}

// generateTOTP generates a TOTP code for a given time step
func (ta *TOTPAuth) generateTOTP(timeStep int64) string {
	// Decode secret key
	key, err := base32.StdEncoding.DecodeString(strings.ToUpper(ta.config.SecretKey))
	if err != nil {
		log.Printf("[%s] Failed to decode secret key: %v", ta.name, err)
		return ""
	}

	// Convert time step to bytes
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(timeStep))

	// Generate HMAC-SHA1
	h := hmac.New(sha1.New, key)
	h.Write(buf)
	hash := h.Sum(nil)

	// Dynamic truncation
	offset := hash[len(hash)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff

	// Generate code
	code := truncated % uint32(pow10(ta.config.CodeDigits))
	format := fmt.Sprintf("%%0%dd", ta.config.CodeDigits)
	return fmt.Sprintf(format, code)
}

// createSession creates a new session and returns the session token
func (ta *TOTPAuth) createSession(req *http.Request) (string, error) {
	// Generate random session token
	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		return "", err
	}
	token := hex.EncodeToString(tokenBytes)

	// Create session
	now := time.Now()
	session := &Session{
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(time.Duration(ta.config.SessionExpiry) * time.Second),
		IP:        ta.getClientIP(req),
	}

	// Store session
	ta.sessions.mu.Lock()
	ta.sessions.sessions[token] = session
	ta.sessions.mu.Unlock()

	return token, nil
}

// cleanupExpiredSessions periodically removes expired sessions
func (ta *TOTPAuth) cleanupExpiredSessions(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			ta.sessions.mu.Lock()
			for token, session := range ta.sessions.sessions {
				if now.After(session.ExpiresAt) {
					delete(ta.sessions.sessions, token)
				}
			}
			ta.sessions.mu.Unlock()
		}
	}
}

// getClientIP extracts the client IP address from the request
func (ta *TOTPAuth) getClientIP(req *http.Request) string {
	// Extract the remote address (direct connection IP)
	remoteIP := req.RemoteAddr
	if idx := strings.LastIndex(remoteIP, ":"); idx != -1 {
		remoteIP = remoteIP[:idx]
	}

	// Parse the remote IP
	ip := net.ParseIP(remoteIP)
	if ip == nil {
		log.Printf("[%s] Failed to parse remote IP: %s", ta.name, remoteIP)
		return remoteIP
	}

	// Check if the request is from a trusted proxy
	isTrustedProxy := false
	for _, network := range ta.trustedNetworks {
		if network.Contains(ip) {
			isTrustedProxy = true
			break
		}
	}

	// If request is from a trusted proxy, check forwarded headers
	if isTrustedProxy {
		// Check X-Forwarded-For header (standard)
		if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
			ips := strings.Split(xff, ",")
			clientIP := strings.TrimSpace(ips[0])
			log.Printf("[%s] Using X-Forwarded-For IP: %s (from trusted proxy %s)", ta.name, clientIP, remoteIP)
			return clientIP
		}

		// Check X-Real-IP header (alternative)
		if xri := req.Header.Get("X-Real-IP"); xri != "" {
			log.Printf("[%s] Using X-Real-IP: %s (from trusted proxy %s)", ta.name, xri, remoteIP)
			return xri
		}
	}

	// Use the direct connection IP (either no trusted proxies configured, or not from trusted proxy)
	return remoteIP
}

// showTOTPPage displays the TOTP input page
func (ta *TOTPAuth) showTOTPPage(rw http.ResponseWriter, req *http.Request, errorMsg string) {
	tmpl := template.Must(template.New("totp").Parse(totpPageTemplate))

	data := map[string]interface{}{
		"Title":       ta.config.PageTitle,
		"Description": ta.config.PageDescription,
		"Error":       errorMsg,
		"Action":      req.URL.String(),
	}

	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.WriteHeader(http.StatusUnauthorized)
	
	if err := tmpl.Execute(rw, data); err != nil {
		log.Printf("[%s] Failed to render TOTP page: %v", ta.name, err)
	}
}

// pow10 calculates 10^n
func pow10(n int) int {
	result := 1
	for i := 0; i < n; i++ {
		result *= 10
	}
	return result
}

// HTML template for TOTP input page
const totpPageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 20px;
        }

        .container {
            background: white;
            border-radius: 16px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
            max-width: 420px;
            width: 100%;
            padding: 40px;
            animation: slideIn 0.4s ease-out;
        }

        @keyframes slideIn {
            from {
                opacity: 0;
                transform: translateY(-20px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }

        .lock-icon {
            width: 80px;
            height: 80px;
            margin: 0 auto 30px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 40px;
        }

        h1 {
            color: #2d3748;
            font-size: 28px;
            font-weight: 700;
            margin-bottom: 12px;
            text-align: center;
        }

        .description {
            color: #718096;
            font-size: 15px;
            line-height: 1.6;
            margin-bottom: 30px;
            text-align: center;
        }

        .error {
            background: #fed7d7;
            border: 1px solid #fc8181;
            color: #c53030;
            padding: 12px 16px;
            border-radius: 8px;
            margin-bottom: 20px;
            font-size: 14px;
            animation: shake 0.4s ease-in-out;
        }

        @keyframes shake {
            0%, 100% { transform: translateX(0); }
            25% { transform: translateX(-10px); }
            75% { transform: translateX(10px); }
        }

        .form-group {
            margin-bottom: 24px;
        }

        label {
            display: block;
            color: #4a5568;
            font-size: 14px;
            font-weight: 600;
            margin-bottom: 8px;
        }

        input[type="text"] {
            width: 100%;
            padding: 14px 16px;
            font-size: 18px;
            border: 2px solid #e2e8f0;
            border-radius: 8px;
            transition: all 0.3s ease;
            font-family: monospace;
            letter-spacing: 4px;
            text-align: center;
        }

        input[type="text"]:focus {
            outline: none;
            border-color: #667eea;
            box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
        }

        button {
            width: 100%;
            padding: 14px 24px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 16px;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.3s ease;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        button:hover {
            transform: translateY(-2px);
            box-shadow: 0 10px 20px rgba(102, 126, 234, 0.3);
        }

        button:active {
            transform: translateY(0);
        }

        .info-text {
            margin-top: 20px;
            padding-top: 20px;
            border-top: 1px solid #e2e8f0;
            color: #718096;
            font-size: 13px;
            text-align: center;
            line-height: 1.6;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="lock-icon">🔒</div>
        <h1>{{.Title}}</h1>
        <p class="description">{{.Description}}</p>
        
        {{if .Error}}
        <div class="error">{{.Error}}</div>
        {{end}}
        
        <form method="POST" action="{{.Action}}">
            <div class="form-group">
                <label for="totp_code">Authentication Code</label>
                <input 
                    type="text" 
                    id="totp_code" 
                    name="totp_code" 
                    maxlength="6" 
                    pattern="[0-9]*"
                    inputmode="numeric"
                    placeholder="000000"
                    autofocus 
                    required
                    autocomplete="off"
                >
            </div>
            <button type="submit">Verify & Continue</button>
        </form>
        
        <div class="info-text">
            Enter the 6-digit code from your authenticator app.<br>
            Codes refresh every 30 seconds.
        </div>
    </div>

    <script>
        document.getElementById('totp_code').focus();
        
        document.getElementById('totp_code').addEventListener('input', function(e) {
            this.value = this.value.replace(/[^0-9]/g, '');
        });
        
        document.getElementById('totp_code').addEventListener('input', function(e) {
            if (this.value.length === 6) {
                this.form.submit();
            }
        });
    </script>
</body>
</html>`