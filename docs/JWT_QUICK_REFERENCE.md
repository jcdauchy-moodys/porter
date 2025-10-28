# JWT RSA256 Quick Reference Card

Quick reference for configuring and using JWT authentication with Porter.

## 📋 Configuration Template

```yaml
auth:
  enabled: true
  type: jwt
  jwt_auth:
    public_key_file: "/path/to/rsa_public.pem"
    issuer: "your-auth-server"
    audience: "porter-api"
```

## 🔑 Generate Keys (One-Time Setup)

```bash
# Generate private key
openssl genrsa -out rsa_private.pem 2048

# Extract public key
openssl rsa -in rsa_private.pem -pubout -out rsa_public.pem

# Set permissions
chmod 400 rsa_private.pem
chmod 444 rsa_public.pem
```

## 🎫 Generate Token (Python)

```python
import jwt
from datetime import datetime, timedelta

# Load private key
with open('rsa_private.pem', 'rb') as f:
    private_key = f.read()

# Create token
payload = {
    'sub': 'user@example.com',
    'exp': datetime.utcnow() + timedelta(hours=1),
    'iss': 'your-auth-server',  # Must match config
    'aud': 'porter-api',         # Must match config
}

token = jwt.encode(payload, private_key, algorithm='RS256')
print(token)
```

## 🚀 Connect to Porter (Python)

```python
from pyarrow import flight

token = "eyJhbGciOiJSUzI1NiIs..."  # Your JWT token

# Connect
client = flight.FlightClient("grpc://localhost:8815")

# Add authentication header
headers = [(b"authorization", f"Bearer {token}".encode())]
options = flight.FlightCallOptions(headers=headers)

# Execute query
query = "SELECT * FROM my_table"
flight_info = client.get_flight_info(
    flight.FlightDescriptor.for_command(query),
    options=options
)

# Get results
reader = client.do_get(flight_info.endpoints[0].ticket, options)
table = reader.read_all()
print(table.to_pandas())
```

## 🎯 Required JWT Claims

| Claim | Required | Description | Example |
|-------|----------|-------------|---------|
| `sub` | ✅ Yes | User identifier | `"user@example.com"` |
| `exp` | ✅ Yes | Expiration (Unix timestamp) | `1735689600` |
| `iss` | ✅ Yes | Issuer (must match config) | `"your-auth-server"` |
| `aud` | ✅ Yes | Audience (must match config) | `"porter-api"` |
| `nbf` | ❌ No | Not before (Unix timestamp) | `1735686000` |
| `iat` | ❌ No | Issued at (Unix timestamp) | `1735686000` |
| `roles` | ❌ No | Custom: user roles | `["admin", "read"]` |

## 🔍 Verify Token (Debug)

```bash
# Decode JWT without verification (for debugging)
echo "YOUR_TOKEN" | cut -d. -f2 | base64 -d | jq .
```

## 🛠️ Common Errors

### "token signature is invalid"
- **Cause**: Wrong public key or token signed with different private key
- **Fix**: Verify key pair matches
```bash
# Both should show same modulus
openssl rsa -pubin -in rsa_public.pem -text -noout | grep -A 20 "modulus"
openssl rsa -in rsa_private.pem -text -noout | grep -A 20 "modulus"
```

### "issuer mismatch" / "audience mismatch"
- **Cause**: JWT claims don't match configuration
- **Fix**: Ensure `iss` and `aud` in token match config values

### "token expired"
- **Cause**: Token `exp` claim is in the past
- **Fix**: Generate new token with future expiration

### "Failed to load JWT public key"
- **Cause**: File not found or invalid PEM format
- **Fix**: Check file path and permissions
```bash
ls -l /path/to/rsa_public.pem
openssl rsa -pubin -in /path/to/rsa_public.pem -text -noout
```

## 📝 Supported Algorithms

| Algorithm | Type | Key Type | Security Level |
|-----------|------|----------|----------------|
| **RS256** | RSA | 2048+ bit | ⭐⭐⭐ Recommended |
| **RS384** | RSA | 3072+ bit | ⭐⭐⭐⭐ High |
| **RS512** | RSA | 4096+ bit | ⭐⭐⭐⭐⭐ Very High |
| **ES256** | ECDSA | P-256 | ⭐⭐⭐⭐ Fast + Secure |
| **ES384** | ECDSA | P-384 | ⭐⭐⭐⭐⭐ Very Secure |
| **ES512** | ECDSA | P-521 | ⭐⭐⭐⭐⭐ Maximum |
| **HS256** | HMAC | Shared Secret | ⭐⭐ Less Secure |

## 🔐 Security Checklist

- [ ] Use 2048-bit (minimum) or 4096-bit RSA keys
- [ ] Store private keys securely (never in git)
- [ ] Set file permissions: `chmod 400` for private keys
- [ ] Use short token expiration (≤ 1 hour)
- [ ] Enable TLS/SSL in production
- [ ] Rotate keys every 90 days
- [ ] Validate `iss` and `aud` claims
- [ ] Monitor authentication failures
- [ ] Backup keys to secure location

## 🎬 Quick Start Script

Use the automated setup script:

```bash
# Run setup script
bash examples/jwt_rsa_setup.sh /etc/porter/keys

# Start Porter
porter server --config /etc/porter/keys/example_config.yaml

# Generate token
python3 /etc/porter/keys/generate_token.py

# Test connection
python3 examples/jwt_flightsql_client.py --token "YOUR_TOKEN"
```

## 📚 Additional Resources

- Full Setup Guide: [`docs/JWT_RSA_SETUP.md`](./JWT_RSA_SETUP.md)
- Middleware README: [`cmd/server/middleware/README.md`](../cmd/server/middleware/README.md)
- Example Config: [`config/jwt_rsa_config.yaml`](../config/jwt_rsa_config.yaml)
- Example Client: [`examples/jwt_flightsql_client.py`](../examples/jwt_flightsql_client.py)
- Setup Script: [`examples/jwt_rsa_setup.sh`](../examples/jwt_rsa_setup.sh)

## 💡 Pro Tips

1. **Token Lifetime**: Use 1 hour or less in production
2. **Key Size**: 2048-bit minimum, 4096-bit for sensitive data
3. **Algorithm Choice**: RS256 is standard, ES256 is faster
4. **Development**: Use HMAC (HS256) for simplicity
5. **Production**: Always use RSA/ECDSA with TLS
6. **Monitoring**: Log auth failures to detect attacks
7. **Caching**: Consider caching validated tokens (not implemented yet)
8. **Multi-tenancy**: Omit `iss`/`aud` in config for flexible validation

## ⚡ One-Liner Examples

```bash
# Generate 4096-bit key pair
openssl genrsa -out key.pem 4096 && openssl rsa -in key.pem -pubout -out pub.pem

# Decode JWT token (without verification)
python3 -c "import sys,jwt; print(jwt.decode(sys.argv[1], options={'verify_signature':False}))" "YOUR_TOKEN"

# Test Porter connection
python3 -c "from pyarrow import flight; c=flight.FlightClient('grpc://localhost:8815'); print(c.list_flights())"
```

---

**Need Help?**
- Open an issue on GitHub
- Check [`docs/JWT_RSA_SETUP.md`](./JWT_RSA_SETUP.md) for detailed guide
- Review test files: [`cmd/server/middleware/auth_test.go`](../cmd/server/middleware/auth_test.go)

