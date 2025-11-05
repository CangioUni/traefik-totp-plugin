# Quick Start Guide

Get your TOTP authentication up and running in 5 minutes!

## Step 1: Generate Your Secret Key

Run the setup helper script:

```bash
python3 setup-totp.py --issuer "MyApp" --account "admin@example.com"
```

This will:
- Generate a random secret key
- Display a QR code in your terminal
- Show configuration snippets
- Provide manual setup instructions

**Example Output:**
```
======================================================================
  TOTP CONFIGURATION GENERATED
======================================================================

📱 SCAN QR CODE:
[QR code displayed here]

🔑 SECRET KEY:
   JBSWY3DPEHPK3PXP

⚙️  TRAEFIK CONFIGURATION:
   Add this to your Traefik dynamic configuration:

   http:
     middlewares:
       totp-auth:
         plugin:
           totp-auth:
             secretKey: "JBSWY3DPEHPK3PXP"
             sessionExpiry: 3600
             issuer: "MyApp"
             accountName: "admin@example.com"
```

## Step 2: Setup Your Authenticator App

### Option A: Scan QR Code
1. Open your authenticator app (Google Authenticator, Authy, Microsoft Authenticator, etc.)
2. Tap "Add Account" or "+"
3. Select "Scan QR Code"
4. Scan the QR code displayed in your terminal

### Option B: Manual Entry
1. Open your authenticator app
2. Choose "Enter a setup key" or "Manual entry"
3. Enter:
   - **Account**: admin@example.com
   - **Key**: JBSWY3DPEHPK3PXP
   - **Type**: Time-based
   - **Digits**: 6

## Step 3: Configure Traefik

### Create Plugin Configuration

Add to your `traefik.yml` (static configuration):

```yaml
experimental:
  plugins:
    totp-auth:
      moduleName: github.com/yourusername/traefik-totp-auth
      version: v1.0.0
```

### Create Middleware Configuration

Add to your `dynamic-config.yml`:

```yaml
http:
  middlewares:
    totp-auth:
      plugin:
        totp-auth:
          secretKey: "JBSWY3DPEHPK3PXP"  # Use your generated secret
          sessionExpiry: 3600
          issuer: "MyApp"
          accountName: "admin@example.com"

  routers:
    my-app:
      rule: "Host(`app.example.com`)"
      service: my-service
      middlewares:
        - totp-auth  # Apply TOTP authentication
      
  services:
    my-service:
      loadBalancer:
        servers:
          - url: "http://localhost:8080"
```

## Step 4: Test It

### Using Docker Compose (Recommended)

1. Navigate to the examples directory:
```bash
cd examples/
```

2. Start the demo:
```bash
docker-compose up -d
```

3. Visit http://localhost in your browser

4. Enter the 6-digit code from your authenticator app

5. You should see the success page!

### Manual Testing

1. Start Traefik with your configuration
2. Access your protected service
3. You'll be prompted for a TOTP code
4. Enter the code from your authenticator app
5. Access granted! 🎉

## Step 5: Production Deployment

### Security Checklist

- [ ] Generate a unique secret key for production
- [ ] Store the secret in environment variables (never in code)
- [ ] Enable `cookieSecure: true` (requires HTTPS)
- [ ] Set appropriate `sessionExpiry` (1-4 hours recommended)
- [ ] Configure `cookieDomain` if using subdomains
- [ ] Test thoroughly before deploying
- [ ] Keep backup codes in a secure location
- [ ] Document the setup for your team

### Environment Variables

Instead of hardcoding secrets, use environment variables:

```yaml
http:
  middlewares:
    totp-auth:
      plugin:
        totp-auth:
          secretKey: "${TOTP_SECRET}"
          sessionExpiry: 3600
          issuer: "${TOTP_ISSUER}"
          accountName: "${TOTP_ACCOUNT}"
```

Set in your environment:
```bash
export TOTP_SECRET="YOUR_SECRET_KEY"
export TOTP_ISSUER="MyApp"
export TOTP_ACCOUNT="admin@example.com"
```

### Docker Compose with Environment Variables

```yaml
services:
  traefik:
    image: traefik:v3.0
    environment:
      - TOTP_SECRET=${TOTP_SECRET}
      - TOTP_ISSUER=${TOTP_ISSUER}
      - TOTP_ACCOUNT=${TOTP_ACCOUNT}
    env_file:
      - .env
```

Create `.env` file (add to `.gitignore`!):
```bash
TOTP_SECRET=JBSWY3DPEHPK3PXP
TOTP_ISSUER=MyApp
TOTP_ACCOUNT=admin@example.com
```

## Troubleshooting

### Problem: "Invalid secret key"
**Solution**: Ensure your secret is valid base32. Generate a new one:
```bash
python3 -c "import base64, os; print(base64.b32encode(os.urandom(20)).decode())"
```

### Problem: Codes don't work
**Solution**: Check server time synchronization:
```bash
# Install NTP
sudo apt-get install ntp
sudo systemctl start ntp

# Or use systemd-timesyncd
sudo timedatectl set-ntp true
```

### Problem: Can't see QR code in terminal
**Solution**: Install qrcode library:
```bash
pip install qrcode[pil]
```

Or generate online (testing only!):
```bash
python3 setup-totp.py --issuer "MyApp" --account "admin"
# Copy the otpauth:// URI and paste into:
# https://www.qr-code-generator.com/
```

### Problem: Session expires too quickly
**Solution**: Increase `sessionExpiry`:
```yaml
sessionExpiry: 7200  # 2 hours instead of 1
```

## Next Steps

- Review the [full README](../README.md) for advanced configuration
- Check out [example configurations](dynamic-config.yml)
- Set up monitoring and alerting for failed authentication attempts
- Consider implementing rate limiting for additional security
- Document the TOTP setup process for your team

## Support

Need help? Check:
- [Full Documentation](../README.md)
- [Example Configurations](dynamic-config.yml)
- [Docker Compose Example](docker-compose.yml)
- GitHub Issues

Happy authenticating! 🔐✨