# JWT Authentication with RSA256 Setup Guide

This guide explains how to configure Porter to use JWT authentication with RSA256 signing algorithm.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Step 1: Generate RSA Key Pair](#step-1-generate-rsa-key-pair)
- [Step 2: Configure Porter](#step-2-configure-porter)
- [Step 3: Create and Sign JWT Tokens](#step-3-create-and-sign-jwt-tokens)
- [Step 4: Test the Connection](#step-4-test-the-connection)
- [Troubleshooting](#troubleshooting)
- [Security Best Practices](#security-best-practices)

## Overview

JWT (JSON Web Token) authentication with RSA256 uses asymmetric cryptography:
- **Private Key**: Used by your authentication server to sign tokens
- **Public Key**: Used by Porter to verify token signatures

This approach is more secure than HMAC (symmetric) because:
- Porter only needs the public key (cannot forge tokens)
- The private key stays with your auth server
- Supports distributed systems better

## Prerequisites

- OpenSSL installed
- Porter server installed
- Access to server filesystem for key storage

## Step 1: Generate RSA Key Pair

### 1.1 Create Keys Directory

```bash
# Create a secure directory for keys
sudo mkdir -p /etc/porter/keys
sudo chmod 700 /etc/porter/keys
```

### 1.2 Generate RSA Private Key

Generate a 2048-bit RSA private key (recommended minimum):

```bash
openssl genrsa -out /etc/porter/keys/rsa_private.pem 2048
```

For higher security, use 4096-bit:

```bash
openssl genrsa -out /etc/porter/keys/rsa_private.pem 4096
```

### 1.3 Extract Public Key

Extract the public key from the private key:

```bash
openssl rsa -in /etc/porter/keys/rsa_private.pem \
            -pubout \
            -out /etc/porter/keys/rsa_public.pem
```

### 1.4 Set Permissions

```bash
# Private key: read-only for owner (if used for token issuance)
sudo chmod 400 /etc/porter/keys/rsa_private.pem

# Public key: readable by porter service
sudo chmod 444 /etc/porter/keys/rsa_public.pem

# Set ownership
sudo chown porter:porter /etc/porter/keys/*.pem
```

### 1.5 Verify Keys

Verify the keys were generated correctly:

```bash
# Display public key
openssl rsa -pubin -in /etc/porter/keys/rsa_public.pem -text -noout

# Display private key (be careful!)
openssl rsa -in /etc/porter/keys/rsa_private.pem -text -noout
```

## Step 2: Configure Porter

### 2.1 Create Configuration File

Create `/etc/porter/config.yaml`:

```yaml
# Porter Server Configuration with JWT RSA256
address: "0.0.0.0:8815"
backend: "duckdb"
database: ":memory:"
log_level: "info"

# JWT Authentication with RSA256
auth:
  enabled: true
  type: jwt
  jwt_auth:
    # Path to RSA public key for verification
    public_key_file: "/etc/porter/keys/rsa_public.pem"
    
    # Issuer validation (must match JWT 'iss' claim)
    issuer: "your-auth-server"
    
    # Audience validation (must match JWT 'aud' claim)
    audience: "porter-api"

metrics:
  enabled: true
  address: ":9090"

health:
  enabled: true
  interval: 10s

reflection: true
```

### 2.2 Alternative: Multiple Signing Methods

You can configure Porter to accept both HMAC and RSA tokens:

```yaml
auth:
  enabled: true
  type: jwt
  jwt_auth:
    # HMAC fallback (for legacy tokens)
    secret: "your-hmac-secret-minimum-32-chars"
    
    # RSA primary method
    public_key_file: "/etc/porter/keys/rsa_public.pem"
    
    issuer: "your-auth-server"
    audience: "porter-api"
```

**Note**: The middleware will automatically detect the signing method from the JWT header.

## Step 3: Create and Sign JWT Tokens

### 3.1 Using Python

Install dependencies:

```bash
pip install pyjwt cryptography
```

Create a token:

```python
import jwt
import datetime
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.backends import default_backend

# Load private key
with open('/etc/porter/keys/rsa_private.pem', 'rb') as key_file:
    private_key = serialization.load_pem_private_key(
        key_file.read(),
        password=None,
        backend=default_backend()
    )

# Create JWT payload
payload = {
    'sub': 'user@example.com',  # Subject (user identifier)
    'exp': datetime.datetime.utcnow() + datetime.timedelta(hours=1),  # Expiration
    'iss': 'your-auth-server',  # Issuer (must match config)
    'aud': 'porter-api',  # Audience (must match config)
    'roles': ['admin', 'read']  # Custom claims
}

# Sign the token
token = jwt.encode(payload, private_key, algorithm='RS256')
print(f"JWT Token: {token}")
```

### 3.2 Using Go

```go
package main

import (
    "crypto/rsa"
    "crypto/x509"
    "encoding/pem"
    "fmt"
    "io/ioutil"
    "time"

    "github.com/golang-jwt/jwt/v5"
)

func main() {
    // Load private key
    keyData, err := ioutil.ReadFile("/etc/porter/keys/rsa_private.pem")
    if err != nil {
        panic(err)
    }
    
    block, _ := pem.Decode(keyData)
    privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
    if err != nil {
        panic(err)
    }

    // Create claims
    claims := jwt.MapClaims{
        "sub": "user@example.com",
        "exp": time.Now().Add(time.Hour).Unix(),
        "iss": "your-auth-server",
        "aud": "porter-api",
        "roles": []string{"admin", "read"},
    }

    // Create and sign token
    token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
    tokenString, err := token.SignedString(privateKey)
    if err != nil {
        panic(err)
    }

    fmt.Printf("JWT Token: %s\n", tokenString)
}
```

### 3.3 Using Node.js

Install dependencies:

```bash
npm install jsonwebtoken
```

Create a token:

```javascript
const jwt = require('jsonwebtoken');
const fs = require('fs');

// Load private key
const privateKey = fs.readFileSync('/etc/porter/keys/rsa_private.pem');

// Create payload
const payload = {
  sub: 'user@example.com',
  exp: Math.floor(Date.now() / 1000) + (60 * 60), // 1 hour
  iss: 'your-auth-server',
  aud: 'porter-api',
  roles: ['admin', 'read']
};

// Sign token
const token = jwt.sign(payload, privateKey, { algorithm: 'RS256' });
console.log('JWT Token:', token);
```

### 3.4 Using Online Tools (Development Only!)

For testing purposes only, you can use [jwt.io](https://jwt.io):

1. Select algorithm: RS256
2. Paste your private key in the "Private Key" field
3. Create your payload:
```json
{
  "sub": "user@example.com",
  "exp": 1735689600,
  "iss": "your-auth-server",
  "aud": "porter-api"
}
```
4. Copy the generated token

**⚠️ WARNING**: Never paste production private keys into online tools!

## Step 4: Test the Connection

### 4.1 Start Porter Server

```bash
# With config file
porter server --config /etc/porter/config.yaml

# Or with environment variables
export PORTER_AUTH_ENABLED=true
export PORTER_AUTH_TYPE=jwt
export PORTER_AUTH_JWT_PUBLIC_KEY_FILE=/etc/porter/keys/rsa_public.pem
export PORTER_AUTH_JWT_ISSUER=your-auth-server
export PORTER_AUTH_JWT_AUDIENCE=porter-api
porter server
```

### 4.2 Test with Python FlightSQL Client

```python
from pyarrow import flight
import pyarrow as pa

# Your JWT token
token = "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."

# Connect to Porter
client = flight.FlightClient("grpc://localhost:8815")

# Create authentication headers
headers = [
    (b"authorization", f"Bearer {token}".encode('utf-8'))
]

# Set up call options with headers
options = flight.FlightCallOptions(headers=headers)

# Test query
query = "SELECT 1 as test"
flight_info = client.get_flight_info(
    flight.FlightDescriptor.for_command(query),
    options=options
)

# Get results
reader = client.do_get(flight_info.endpoints[0].ticket, options=options)
table = reader.read_all()
print(table)
```

### 4.3 Test with cURL (via gRPC-Web or REST proxy)

If you have a gRPC-Web proxy:

```bash
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     -X POST \
     http://localhost:8815/flight.sql.FlightSQLService/GetFlightInfo \
     -d '{"query": "SELECT 1"}'
```

## Troubleshooting

### Issue: "Invalid JWT: token signature is invalid"

**Causes**:
- Wrong public key configured
- Token signed with different private key
- Key format issues

**Solutions**:
```bash
# Verify keys match
# This should output the same modulus for both
openssl rsa -pubin -in /etc/porter/keys/rsa_public.pem -text -noout | grep -A 20 "modulus"
openssl rsa -in /etc/porter/keys/rsa_private.pem -text -noout | grep -A 20 "modulus"
```

### Issue: "Issuer mismatch" or "Audience mismatch"

**Solution**: Ensure JWT claims match configuration:
- JWT `iss` claim = config `issuer`
- JWT `aud` claim = config `audience`

Check token contents:
```bash
# Decode JWT (doesn't verify signature)
echo "YOUR_TOKEN" | cut -d. -f2 | base64 -d | jq .
```

### Issue: "Token expired"

**Solution**: Generate a new token with future expiration:
```python
'exp': datetime.datetime.utcnow() + datetime.timedelta(hours=1)
```

### Issue: "Failed to load JWT public key"

**Causes**:
- File doesn't exist
- Permission denied
- Invalid PEM format

**Solutions**:
```bash
# Check file exists
ls -l /etc/porter/keys/rsa_public.pem

# Check permissions
sudo chmod 444 /etc/porter/keys/rsa_public.pem

# Verify PEM format
openssl rsa -pubin -in /etc/porter/keys/rsa_public.pem -text -noout
```

### Issue: "Unexpected HMAC token" or "Unexpected asymmetric key token"

**Cause**: Token algorithm doesn't match configured keys

**Solution**: Ensure:
- If using RSA: Token signed with RS256/RS384/RS512
- If using HMAC: Token signed with HS256/HS384/HS512
- Configure both `secret` and `public_key_file` to accept both

## Security Best Practices

### 1. Key Management

- **Store private keys securely**: Use a secrets manager (AWS Secrets Manager, HashiCorp Vault)
- **Rotate keys regularly**: Implement key rotation every 90 days
- **Use strong key sizes**: 2048-bit minimum, 4096-bit recommended
- **Never commit keys to git**: Add `*.pem` to `.gitignore`

### 2. Token Lifetime

- **Short expiration**: 1 hour or less for production
- **Implement refresh tokens**: For longer sessions
- **Include `nbf` (not before)**: Prevent premature token use

```python
payload = {
    'sub': 'user@example.com',
    'exp': datetime.datetime.utcnow() + datetime.timedelta(hours=1),
    'nbf': datetime.datetime.utcnow(),  # Not valid before now
    'iss': 'your-auth-server',
    'aud': 'porter-api',
}
```

### 3. Transport Security

- **Always use TLS/SSL**: Enable TLS in Porter config
- **Validate certificates**: Don't skip certificate verification

```yaml
tls:
  enabled: true
  cert_file: "/etc/porter/tls/server.crt"
  key_file: "/etc/porter/tls/server.key"
```

### 4. Claim Validation

- **Always validate issuer**: Prevents token reuse across services
- **Always validate audience**: Prevents token misuse
- **Include custom claims**: Add `roles`, `permissions` for authorization

### 5. Monitoring

- **Log authentication failures**: Monitor for brute force attacks
- **Track token usage**: Detect anomalies
- **Set up alerts**: For repeated authentication failures

### 6. Key Backup and Recovery

```bash
# Backup keys securely
sudo tar -czf porter-keys-backup-$(date +%Y%m%d).tar.gz \
    /etc/porter/keys/

# Encrypt backup
gpg -c porter-keys-backup-20240101.tar.gz

# Store in secure location
aws s3 cp porter-keys-backup-20240101.tar.gz.gpg \
    s3://your-secure-bucket/backups/
```

## Advanced Configuration

### Multi-Tenancy

Support different issuers/audiences:

```yaml
auth:
  enabled: true
  type: jwt
  jwt_auth:
    public_key_file: "/etc/porter/keys/rsa_public.pem"
    # Don't specify issuer/audience for flexible validation
    # Validate in your application logic instead
```

### Key Rotation

1. Generate new key pair
2. Update auth server to sign with new key
3. Configure Porter with both old and new public keys
4. After all tokens expire, remove old key

### Performance Tuning

For high-throughput scenarios:
- Use ECDSA (ES256) instead of RSA (faster verification)
- Cache validated tokens (not implemented yet)
- Use shorter keys in development (faster generation)

## Additional Resources

- [JWT.io](https://jwt.io) - JWT debugger
- [RFC 7519](https://tools.ietf.org/html/rfc7519) - JWT specification
- [RFC 7518](https://tools.ietf.org/html/rfc7518) - JSON Web Algorithms
- [Porter Documentation](../README.md)
- [Middleware README](../cmd/server/middleware/README.md)

## Support

For issues or questions:
- Open an issue on GitHub
- Check existing documentation in `docs/`
- Review test files in `cmd/server/middleware/auth_test.go`

