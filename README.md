# Traefik TOTP Authentication Plugin

A Traefik middleware plugin that adds TOTP (Time-based One-Time Password) authentication to your services. Protect your applications with two-factor authentication using standard authenticator apps like Google Authenticator, Authy, or Microsoft Authenticator.

## Features

- 🔒 **TOTP Authentication**: RFC 6238 compliant time-based one-time passwords
- 💾 **In-Memory Sessions**: Fast session management with configurable expiration
- 🎨 **Beautiful UI**: Modern, responsive authentication page
- ⚡ **Auto-Submit**: Automatically submits when 6 digits are entered
- 🕐 **Clock Skew Tolerance**: Handles time synchronization issues
- 🔐 **Secure Cookies**: HttpOnly, Secure, SameSite protection
- 📱 **Mobile Friendly**: Works great on phones and tablets
- 🌐 **Optional IP Validation**: Optionally tie sessions to IP addresses for extra security

## Installation

### Using Traefik Pilot (Recommended)

⚠️ The integration with Traefik Pilot is not verified
Add the plugin to your Traefik static configuration:

```yaml
# traefik.yml
experimental:
  plugins:
    totp-auth:
      moduleName: github.com/CangioUni/traefik-totp-plugin
      version: v0.1.0
```

### Local Development
Add the plugin to your Traefik static configuration:

```yaml
# traefik.yml
experimental:
  localPlugins:
    totp-auth:
      moduleName: github.com/CangioUni/traefik-totp-plugin
```

```yaml
cd traefik/plugins
git clone https://github.com/CangioUni/traefik-totp-plugin.git
```

## Configuration

### Generate a TOTP Secret

First, you need to generate a TOTP secret key. You can use an online generator like: https://totp.danhersam.com/

Otherwise, if you have Python installed, run this command:

```bash
# Using Python
python3 -c "import base64, os; print(base64.b32encode(os.urandom(20)).decode())"
```

This will output something like: `JBSWY3DPEHPK3PXP`

### Basic Configuration

Add the middleware to your Traefik dynamic configuration:

```yaml
# dynamic-config.yml
http:
  middlewares:
    totp-auth:
      plugin:
        totp-auth:
          secretKey: "JBSWY3DPEHPK3PXP"  # Your base32 encoded secret
          sessionExpiry: 3600             # 1 hour (in seconds)
          issuer: "MyApp"                 # Name shown in authenticator app
          accountName: "user@example.com" # Account name in authenticator app
```

### Advanced Configuration

```yaml
http:
  middlewares:
    totp-auth:
      plugin:
        totp-auth:
          secretKey: "JBSWY3DPEHPK3PXP"
          sessionExpiry: 7200              # 2 hours
          cookieName: "my_totp_session"
          cookieDomain: ".example.com"
          cookieSecure: true
          issuer: "MyCompany"
          accountName: "admin@company.com"
          timeStep: 30                     # Time step in seconds
          codeDigits: 6                    # Number of digits in code
          allowedSkew: 1                   # Allow ±1 time step for clock skew
          validateIP: false                # Set to true for stricter security (may break with proxies)
          pageTitle: "Secure Access Required"
          pageDescription: "Enter your authentication code to continue"
```

### Apply to a Route

```yaml
http:
  routers:
    secure-app:
      rule: "Host(`app.example.com`)"
      service: my-service
      middlewares:
        - totp-auth  # Add TOTP authentication
      
  services:
    my-service:
      loadBalancer:
        servers:
          - url: "http://localhost:8080"
```

## Setting Up TOTP on Your Device

### Method 1: QR Code (Easiest)

1. Generate a QR code with your secret key:

```bash
# Install qrencode (if not already installed)
# Ubuntu/Debian: apt-get install qrencode
# macOS: brew install qrencode

# Generate QR code
SECRET="JBSWY3DPEHPK3PXP"
ISSUER="MyApp"
ACCOUNT="user@example.com"

qrencode -t ANSIUTF8 "otpauth://totp/${ISSUER}:${ACCOUNT}?secret=${SECRET}&issuer=${ISSUER}"
```

2. Scan the QR code with your authenticator app
3. The app will start generating codes

### Method 2: Manual Entry

1. Open your authenticator app (Google Authenticator, Authy, etc.)
2. Choose "Enter a setup key" or "Manual entry"
3. Enter these details:
   - **Account name**: user@example.com (or whatever you configured)
   - **Secret key**: JBSWY3DPEHPK3PXP (your secret)
   - **Type**: Time-based
   - **Algorithm**: SHA1
   - **Digits**: 6
   - **Period**: 30 seconds

### Method 3: Using a Python Script

```python
#!/usr/bin/env python3
import qrcode

secret = "JBSWY3DPEHPK3PXP"
issuer = "MyApp"
account = "user@example.com"

uri = f"otpauth://totp/{issuer}:{account}?secret={secret}&issuer={issuer}"

qr = qrcode.QRCode(version=1, box_size=10, border=5)
qr.add_data(uri)
qr.make(fit=True)
qr.print_ascii()

print(f"\nOr enter manually:")
print(f"Secret: {secret}")
print(f"Account: {account}")
print(f"Issuer: {issuer}")
```

### Method 4: Online QR Generator (Use with Caution)

**⚠️ Warning**: Only use this for testing, never for production secrets!

Visit: https://www.qr-code-generator.com/
- Input: `otpauth://totp/MyApp:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=MyApp`
- Generate and scan the QR code

## Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `secretKey` | string | **required** | Base32 encoded TOTP secret key |
| `sessionExpiry` | int | 3600 | Session duration in seconds (1 hour default) |
| `cookieName` | string | "totp_session" | Name of the session cookie |
| `cookieDomain` | string | "" | Cookie domain (empty = current domain) |
| `cookieSecure` | bool | true | Use secure cookies (HTTPS only) |
| `issuer` | string | "" | Issuer name shown in authenticator app |
| `accountName` | string | "" | Account name shown in authenticator app |
| `timeStep` | int | 30 | TOTP time step in seconds |
| `codeDigits` | int | 6 | Number of digits in TOTP code |
| `allowedSkew` | int | 1 | Number of time steps to allow for clock skew |
| `pageTitle` | string | "TOTP Authentication Required" | Custom page title |
| `pageDescription` | string | "Please enter your TOTP code..." | Custom page description |
| `validateIP` | bool | false | Enable IP validation for sessions (may break with proxies/NAT) |

## How It Works

1. **First Visit**: User accesses a protected resource
2. **Authentication Page**: Plugin displays a beautiful TOTP input page
3. **Code Entry**: User enters the 6-digit code from their authenticator app
4. **Validation**: Plugin validates the code against the secret key
5. **Session Created**: On success, plugin creates a session cookie
6. **Access Granted**: User can now access the protected resource
7. **Session Expiry**: After the configured time, user must re-authenticate

## Security Features

- **In-Memory Sessions**: Sessions are stored in memory only (not persisted to disk)
- **Optional IP Validation**: Optionally tie sessions to IP addresses (disabled by default for compatibility)
- **HttpOnly Cookies**: Session cookies are not accessible via JavaScript
- **Secure Cookies**: Cookies only sent over HTTPS (configurable)
- **SameSite Protection**: CSRF protection via SameSite cookie attribute
- **Clock Skew Tolerance**: Accepts codes from ±1 time window (configurable)
- **Auto Cleanup**: Expired sessions are automatically removed every 5 minutes

## Testing

### Test with Docker Compose

```yaml
# docker-compose.yml
version: '3'

services:
  traefik:
    image: traefik:v3.0
    command:
      - --api.insecure=true
      - --providers.file.filename=/config/dynamic.yml
      - --experimental.plugins.totp-auth.modulename=github.com/yourusername/traefik-totp-auth
      - --experimental.plugins.totp-auth.version=v1.0.0
    ports:
      - "80:80"
      - "8080:8080"
    volumes:
      - ./config:/config

  whoami:
    image: traefik/whoami
```

```yaml
# config/dynamic.yml
http:
  middlewares:
    totp:
      plugin:
        totp-auth:
          secretKey: "JBSWY3DPEHPK3PXP"
          sessionExpiry: 3600
          issuer: "TestApp"
          accountName: "test@example.com"

  routers:
    whoami:
      rule: "Host(`localhost`)"
      service: whoami
      middlewares:
        - totp

  services:
    whoami:
      loadBalancer:
        servers:
          - url: "http://whoami"
```

### Manual Testing

1. Generate a test secret: `JBSWY3DPEHPK3PXP`
2. Add to your authenticator app
3. Configure the plugin with this secret
4. Visit your protected URL
5. Enter the code from your authenticator app
6. Verify access is granted and session persists

## Troubleshooting

### "Invalid secret key" error
- Ensure your secret is properly base32 encoded
- Remove any spaces or special characters
- Valid characters: A-Z and 2-7

### Codes not working
- Check that your server time is synchronized (use NTP)
- Try increasing `allowedSkew` to 2 or 3
- Verify the secret key matches in both plugin and authenticator app

### Session expires too quickly
- Increase `sessionExpiry` value (in seconds)
- Default is 3600 seconds (1 hour)

### Cookie not being set
- Ensure `cookieSecure: false` if testing without HTTPS
- Check browser console for cookie errors
- Verify `cookieDomain` is correctly set (or empty)

### Session expires immediately on page refresh
- IP validation is disabled by default to prevent this issue
- If you enabled `validateIP: true` and users are behind proxies/NAT, their IP may change between requests
- Solution: Keep `validateIP: false` (default) or ensure X-Forwarded-For headers are stable
- Only enable IP validation in controlled environments with stable client IPs

## Example: Complete Setup

1. **Generate Secret**:
```bash
python3 -c "import base64, os; print(base64.b32encode(os.urandom(20)).decode())"
# Output: N3V2IY4HLQHC6VKF2LJNBXAU6M
```

2. **Configure Plugin**:
```yaml
http:
  middlewares:
    my-totp:
      plugin:
        totp-auth:
          secretKey: "N3V2IY4HLQHC6VKF2LJNBXAU6M"
          sessionExpiry: 3600
          issuer: "MyCompany Portal"
          accountName: "admin"
```

3. **Setup Authenticator**:
```bash
# Generate QR code
qrencode -t ANSIUTF8 "otpauth://totp/MyCompany%20Portal:admin?secret=N3V2IY4HLQHC6VKF2LJNBXAU6M&issuer=MyCompany%20Portal"
```

4. **Test**:
- Visit your protected URL
- Enter code from authenticator app
- Enjoy secure access for 1 hour

## Best Practices

1. **Secret Key Security**: Never commit secrets to version control
2. **Use Environment Variables**: Store secrets in environment variables
3. **HTTPS Only**: Always use `cookieSecure: true` in production
4. **Regular Rotation**: Consider rotating secrets periodically
5. **Backup Codes**: Implement backup authentication methods
6. **Monitor Failed Attempts**: Log and monitor authentication failures
7. **Session Duration**: Balance security with user convenience (1-4 hours typical)

## License

MIT License - See LICENSE file for details

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## Support

For issues, questions, or contributions, please visit the GitHub repository.
