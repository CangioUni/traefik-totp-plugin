#!/usr/bin/env python3
"""
TOTP Setup Helper Script
Generates TOTP secrets and QR codes for easy configuration
"""

import base64
import os
import sys
import argparse


def generate_secret():
    """Generate a random base32 encoded secret"""
    random_bytes = os.urandom(20)
    secret = base64.b32encode(random_bytes).decode('utf-8')
    return secret


def generate_uri(secret, issuer, account):
    """Generate otpauth URI"""
    return f"otpauth://totp/{issuer}:{account}?secret={secret}&issuer={issuer}"


def print_qr_terminal(uri):
    """Print QR code to terminal using qrcode library"""
    try:
        import qrcode
        qr = qrcode.QRCode(version=1, box_size=1, border=2)
        qr.add_data(uri)
        qr.make(fit=True)
        qr.print_ascii()
    except ImportError:
        print("\n⚠️  qrcode library not installed. Install with: pip install qrcode")
        print("   Or use the URI below to generate a QR code online.\n")


def print_instructions(secret, issuer, account, uri):
    """Print setup instructions"""
    print("\n" + "="*70)
    print("  TOTP CONFIGURATION GENERATED")
    print("="*70)
    
    print("\n📱 SCAN QR CODE:")
    print_qr_terminal(uri)
    
    print("\n🔑 SECRET KEY:")
    print(f"   {secret}")
    
    print("\n📋 MANUAL ENTRY DETAILS:")
    print(f"   Account: {account}")
    print(f"   Issuer:  {issuer}")
    print(f"   Secret:  {secret}")
    print(f"   Type:    Time-based")
    print(f"   Digits:  6")
    print(f"   Period:  30 seconds")
    
    print("\n🔗 OTPAUTH URI:")
    print(f"   {uri}")
    
    print("\n⚙️  TRAEFIK CONFIGURATION:")
    print("   Add this to your Traefik dynamic configuration:\n")
    print("   http:")
    print("     middlewares:")
    print("       totp-auth:")
    print("         plugin:")
    print("           totp-auth:")
    print(f'             secretKey: "{secret}"')
    print("             sessionExpiry: 3600")
    print(f'             issuer: "{issuer}"')
    print(f'             accountName: "{account}"')
    
    print("\n💡 TIPS:")
    print("   • Keep the secret key secure and never commit it to version control")
    print("   • Use environment variables for production deployments")
    print("   • Test the setup before deploying to production")
    print("   • Save backup codes in a secure location")
    
    print("\n" + "="*70 + "\n")


def main():
    parser = argparse.ArgumentParser(
        description='Generate TOTP configuration for Traefik plugin',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s --issuer "MyApp" --account "admin@example.com"
  %(prog)s --issuer "Company Portal" --account "user" --secret "EXISTING_SECRET"
        """
    )
    
    parser.add_argument(
        '--issuer',
        default='MyService',
        help='Issuer name (shown in authenticator app)'
    )
    
    parser.add_argument(
        '--account',
        default='user@example.com',
        help='Account name (shown in authenticator app)'
    )
    
    parser.add_argument(
        '--secret',
        help='Use existing secret instead of generating new one'
    )
    
    args = parser.parse_args()
    
    # Generate or use provided secret
    if args.secret:
        secret = args.secret.upper().replace(' ', '')
        # Validate secret
        try:
            base64.b32decode(secret)
        except Exception:
            print("❌ Error: Invalid secret key. Must be valid base32 encoding.")
            sys.exit(1)
    else:
        secret = generate_secret()
    
    # Generate URI
    uri = generate_uri(secret, args.issuer, args.account)
    
    # Print instructions
    print_instructions(secret, args.issuer, args.account, uri)


if __name__ == '__main__':
    main()