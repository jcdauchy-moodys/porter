# Porter Middleware

This package provides authentication and authorization middleware for the Porter server. It supports multiple authentication methods and integrates with gRPC interceptors for both unary and streaming RPCs.

## Features

- Multiple authentication methods:
  - Basic Authentication
  - Bearer Token Authentication
  - JWT Authentication (HMAC, RSA, and ECDSA)
  - OAuth2 Authentication
- Role-based access control
- Health check endpoint bypass
- Configurable authentication settings
- Comprehensive test coverage

## Authentication Methods

### Basic Authentication

Basic authentication uses username/password pairs stored in the configuration. Passwords are stored as plain text in the configuration file.

```yaml
auth:
  enabled: true
  type: basic
  basic_auth:
    users:
      admin:
        password: "admin123"
        roles: ["admin"]
      user:
        password: "user123"
        roles: ["user"]
```

### Bearer Token Authentication

Bearer token authentication uses predefined tokens mapped to users. Each token is associated with a specific user.

```yaml
auth:
  enabled: true
  type: bearer
  bearer_auth:
    tokens:
      "token1": "user1"
      "token2": "user2"
```

### JWT Authentication

JWT authentication supports multiple signing methods:

- HMAC (HS256, HS384, HS512)
- RSA (RS256, RS384, RS512)
- ECDSA (ES256, ES384, ES512)

#### HMAC Configuration (HS256)

For symmetric key signing (shared secret):

```yaml
auth:
  enabled: true
  type: jwt
  jwt_auth:
    secret: "your-secret-key-minimum-32-chars"
    issuer: "your-issuer"
    audience: "your-audience"
```

#### RSA Configuration (RS256)

For asymmetric key signing (public/private key pair):

```yaml
auth:
  enabled: true
  type: jwt
  jwt_auth:
    public_key_file: "/path/to/rsa_public.pem"
    issuer: "your-issuer"
    audience: "your-audience"
```

If you also want to issue tokens (not just validate them):

```yaml
auth:
  enabled: true
  type: jwt
  jwt_auth:
    private_key_file: "/path/to/rsa_private.pem"
    issuer: "your-issuer"
    audience: "your-audience"
```

#### ECDSA Configuration (ES256)

For elliptic curve signing:

```yaml
auth:
  enabled: true
  type: jwt
  jwt_auth:
    public_key_file: "/path/to/ecdsa_public.pem"
    issuer: "your-issuer"
    audience: "your-audience"
```

#### Generating RSA Keys

Generate an RSA key pair for RS256:

```bash
# Generate private key (2048-bit)
openssl genrsa -out rsa_private.pem 2048

# Extract public key
openssl rsa -in rsa_private.pem -pubout -out rsa_public.pem

# For 4096-bit (more secure)
openssl genrsa -out rsa_private.pem 4096
openssl rsa -in rsa_private.pem -pubout -out rsa_public.pem
```

#### Generating ECDSA Keys

Generate an ECDSA key pair for ES256:

```bash
# Generate private key (P-256 curve)
openssl ecparam -name prime256v1 -genkey -noout -out ecdsa_private.pem

# Extract public key
openssl ec -in ecdsa_private.pem -pubout -out ecdsa_public.pem
```

#### JWT Token Requirements

All JWT tokens must include:
- `sub` (subject): User identifier
- `exp` (expiration): Token expiration time (Unix timestamp)
- `iss` (issuer): Must match configured issuer
- `aud` (audience): Must match configured audience

Example JWT payload:
```json
{
  "sub": "user@example.com",
  "exp": 1735689600,
  "iss": "porter-server",
  "aud": "porter-client",
  "roles": ["admin", "read"]
}
```

### OAuth2 Authentication

OAuth2 authentication supports:

- Authorization Code flow
- Client Credentials flow
- Refresh Token flow

```yaml
auth:
  enabled: true
  type: oauth2
  oauth2_auth:
    clients:
      client1:
        secret: "client1secret"
        redirect_uris: ["http://localhost:8080/callback"]
        grant_types: ["authorization_code", "refresh_token"]
    token_expiry: 3600  # in seconds
    refresh_token_expiry: 604800  # in seconds
```

## Usage

The middleware can be used with both unary and streaming gRPC interceptors:

```go
// Create middleware instance
middleware := NewAuthMiddleware(config, logger)

// Use with unary interceptor
server := grpc.NewServer(
    grpc.UnaryInterceptor(middleware.UnaryInterceptor()),
)

// Use with stream interceptor
server := grpc.NewServer(
    grpc.StreamInterceptor(middleware.StreamInterceptor()),
)
```

## Context Values

The middleware adds the following values to the context:

- `UserKey`: The authenticated username
- `RolesKey`: The user's roles

These can be accessed using the provided helper functions:

```go
user := AuthenticatedUser(ctx)
roles := AuthenticatedRoles(ctx)
```

## Health Check Bypass

The middleware automatically bypasses authentication for health check endpoints:

- `/grpc.health.v1.Health/Check`
- `/grpc.health.v1.Health/Watch`

## Security Considerations

1. **Basic Auth**:
   - Use HTTPS in production
   - Consider using a more secure authentication method
   - Regularly rotate passwords

2. **Bearer Tokens**:
   - Use strong, random tokens
   - Implement token rotation
   - Store tokens securely

3. **JWT**:
   - Use appropriate key sizes
   - Set reasonable expiration times
   - Validate all claims
   - Use secure signing algorithms

4. **OAuth2**:
   - Use HTTPS for all endpoints
   - Implement proper token storage
   - Validate all redirect URIs
   - Use secure client secrets

## Testing

The package includes comprehensive tests for all authentication methods and edge cases. Run the tests using:

```bash
go test -v ./...
```

## Contributing

When adding new authentication methods or features:

1. Add appropriate configuration options
2. Implement the authentication logic
3. Add comprehensive tests
4. Update this documentation
5. Follow the existing code style and patterns
