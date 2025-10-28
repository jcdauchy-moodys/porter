# Porter Server Architecture

## Overview

Porter has two server implementations that serve different purposes: a **Standard Server** for core functionality and an **Enterprise Server** for production deployments.

## Two Server Implementations

### 1. Standard Server (`pkg/server/flight_sql.go`)

This is the **basic, clean implementation** of the Flight SQL server.

**Location**: `pkg/server/flight_sql.go`

**Characteristics**:
- ✅ Simple, straightforward implementation
- ✅ All core Flight SQL functionality
- ✅ Query execution, metadata discovery, transactions, prepared statements
- ✅ No enterprise features (metrics, caching, middleware)
- ✅ Suitable for development, testing, simple deployments
- ✅ Easier to understand and maintain

**Use case**: When you want a simple Flight SQL server without bells and whistles.

### 2. Enterprise Server (`cmd/server/main.go`)

This is the **production-grade implementation** with enterprise features.

**Location**: `cmd/server/main.go` (the `EnterpriseFlightSQLServer` struct)

**Characteristics**:
- ✅ All standard server features **PLUS**:
- 🚀 **Metrics collection** (Prometheus)
- 🚀 **Request caching** (in-memory cache for query results)
- 🚀 **Authentication & Authorization** (Basic, Bearer, JWT, OAuth2, mTLS)
- 🚀 **Middleware stack** (logging, recovery, metrics, auth)
- 🚀 **TLS/mTLS support**
- 🚀 **Health checks** with custom queries per backend
- 🚀 **Performance monitoring** (timers, request tracking)
- 🚀 **Graceful shutdown**
- 🚀 **Connection pooling** with circuit breakers
- 🚀 **Advanced error handling**

**Use case**: Production deployments where you need observability, security, and reliability.

## Code Structure

```
Porter/
├── pkg/server/flight_sql.go          # Standard server (core functionality)
│   └── FlightSQLServer struct
│
└── cmd/server/main.go                 # Enterprise server (production features)
    └── EnterpriseFlightSQLServer struct
        ├── Embeds: flightsql.BaseServer
        ├── + Metrics collector
        ├── + Memory cache
        ├── + Authentication middleware
        ├── + Logging middleware
        ├── + Recovery middleware
        └── + Custom handlers with enterprise features
```

## Feature Comparison

| Feature | Standard Server | Enterprise Server |
|---------|----------------|-------------------|
| **Query Execution** | ✅ Yes | ✅ Yes |
| **Metadata Discovery** | ✅ Yes | ✅ Yes |
| **Transactions** | ✅ Yes | ✅ Yes |
| **Prepared Statements** | ✅ Yes | ✅ Yes |
| **Metrics** | ❌ No | ✅ Prometheus metrics |
| **Caching** | ❌ No | ✅ In-memory cache |
| **Authentication** | ❌ No | ✅ Multiple methods |
| **Middleware** | ❌ No | ✅ Full middleware stack |
| **TLS** | ❌ No | ✅ TLS/mTLS support |
| **Request Logging** | ❌ Basic | ✅ Structured logging |
| **Error Recovery** | ❌ Basic | ✅ Panic recovery |
| **Performance Tracking** | ❌ No | ✅ Per-request timers |

## Flight SQL Protocol Implementation Status

### Query Execution Methods

| Method | Standard Server | Enterprise Server | Status |
|--------|----------------|-------------------|--------|
| `GetFlightInfoStatement` | ✅ Implemented | ✅ Implemented | Complete |
| `DoGetStatement` | ✅ Implemented | ✅ Implemented | Complete |
| `DoPutCommandStatementUpdate` | ✅ Implemented | ✅ Implemented | Complete |

### Prepared Statement Methods

| Method | Standard Server | Enterprise Server | Status |
|--------|----------------|-------------------|--------|
| `CreatePreparedStatement` | ✅ Implemented | ✅ Implemented | Complete |
| `ClosePreparedStatement` | ✅ Implemented | ✅ Implemented | Complete |
| `GetFlightInfoPreparedStatement` | ✅ Implemented | ✅ Implemented | Complete |
| `DoGetPreparedStatement` | ✅ Implemented | ✅ Implemented | Complete |
| `DoPutPreparedStatementQuery` | ✅ Implemented | ✅ Implemented | Complete |
| `DoPutPreparedStatementUpdate` | ✅ Implemented | ✅ Implemented | Complete |

### Transaction Methods

| Method | Standard Server | Enterprise Server | Status |
|--------|----------------|-------------------|--------|
| `BeginTransaction` | ✅ Implemented | ✅ Implemented | Complete |
| `EndTransaction` | ✅ Implemented | ✅ Implemented | Complete |

### Metadata Discovery Methods

| Method | Standard Server | Enterprise Server | Status |
|--------|----------------|-------------------|--------|
| `GetFlightInfoCatalogs` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `DoGetCatalogs` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `GetFlightInfoSchemas` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `DoGetDBSchemas` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `GetFlightInfoTables` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `DoGetTables` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `GetFlightInfoTableTypes` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `DoGetTableTypes` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `GetFlightInfoPrimaryKeys` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `DoGetPrimaryKeys` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `GetFlightInfoImportedKeys` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `DoGetImportedKeys` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `GetFlightInfoExportedKeys` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `DoGetExportedKeys` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `GetFlightInfoCrossReference` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `DoGetCrossReference` | ✅ Implemented | ✅ Implemented | ✅ Complete |

### SQL Info Methods

| Method | Standard Server | Enterprise Server | Status |
|--------|----------------|-------------------|--------|
| `GetFlightInfoSqlInfo` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `DoGetSqlInfo` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `GetFlightInfoXdbcTypeInfo` | ✅ Implemented | ✅ Implemented | ✅ Complete |
| `DoGetXdbcTypeInfo` | ✅ Implemented | ✅ Implemented | ✅ Complete |

### Additional Flight Methods

| Method | Standard Server | Enterprise Server | Status |
|--------|----------------|-------------------|--------|
| `DoGet` | ✅ Implemented | ✅ Implemented | Complete |
| `Handshake` | ✅ Implemented | ✅ Implemented | Complete |
| `GetSchema` | ✅ Inherited | ✅ Inherited | Complete |
| `ListFlights` | ✅ Inherited | ✅ Inherited | Complete |
| `ListActions` | ✅ Inherited | ✅ Inherited | Complete |
| `DoAction` | ✅ Inherited | ✅ Inherited | Complete |
| `DoPut` | ✅ Inherited | ✅ Inherited | Complete |
| `DoExchange` | ✅ Inherited | ✅ Inherited | Complete |

## Implementation Notes

### Recent Updates (2025-10-28)

1. **Added all metadata discovery methods to Enterprise Server**:
   - Previously, the Enterprise Server was missing these methods
   - Now has full parity with Standard Server
   - Includes metrics tracking for all metadata operations

2. **Oracle Backend Support**:
   - Health check query defaults to `SELECT 1 FROM DUAL` for Oracle
   - Configurable health queries per backend
   - Oracle configuration properly loaded from YAML files

3. **Configuration Improvements**:
   - Added `mapstructure` tags to all config structs
   - Fixed config unmarshalling from YAML files
   - Removed deprecated SID support (service_name only)

## Why Two Implementations?

### 1. Separation of Concerns
- Core SQL logic (`pkg/server`) stays clean and testable
- Enterprise features (`cmd/server`) are layered on top
- Easy to test core functionality in isolation

### 2. Flexibility
- Can use standard server for simple use cases
- Can build custom wrappers with different features
- Not forced to include enterprise overhead if not needed

### 3. Maintenance
- Core functionality is easier to test and update
- Enterprise features can be added/removed without affecting core
- Clear boundaries between protocol implementation and production features

## What Porter Uses

When you run Porter via `cmd/server/main.go`, you're using the **Enterprise Server**. This gives you:

- ✅ Oracle health checks with `SELECT 1 FROM DUAL`
- ✅ Metrics endpoint at `:9090/metrics`
- ✅ Configuration file support (YAML)
- ✅ TLS and authentication options
- ✅ Proper logging and error handling
- ✅ Request caching for better performance
- ✅ Graceful shutdown
- ✅ Full metadata discovery support

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                     Client (FlightSqlClient)                     │
└───────────────────────────────┬─────────────────────────────────┘
                                │ Flight SQL Protocol
                                │
┌───────────────────────────────▼─────────────────────────────────┐
│              Enterprise Flight SQL Server (main.go)              │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    Middleware Stack                        │  │
│  │  • Recovery  • Logging  • Metrics  • Authentication       │  │
│  └───────────────────────────┬───────────────────────────────┘  │
│                              │                                   │
│  ┌───────────────────────────▼───────────────────────────────┐  │
│  │              Flight SQL Protocol Handler                   │  │
│  │  • Query Execution    • Prepared Statements               │  │
│  │  • Transactions       • Metadata Discovery                │  │
│  └───────────────────────────┬───────────────────────────────┘  │
│                              │                                   │
│  ┌──────────────┬────────────┴──────────┬──────────────────┐   │
│  │              │                       │                  │   │
│  ▼              ▼                       ▼                  ▼   │
│ Cache    Query Handler      Metadata Handler    Transaction    │
│                │                       │              Handler   │
└────────────────┼───────────────────────┼───────────────────────┘
                 │                       │
                 ▼                       ▼
        ┌────────────────────────────────────────┐
        │      Connection Pool with              │
        │    Circuit Breaker & Retry             │
        └────────────────┬───────────────────────┘
                         │
                         ▼
        ┌────────────────────────────────────────┐
        │     Database Backend                   │
        │  • DuckDB  • Oracle  • ClickHouse      │
        └────────────────────────────────────────┘
```

## Performance Considerations

### Enterprise Server Overhead

The Enterprise Server adds minimal overhead:
- **Metrics**: ~0.1ms per request
- **Logging**: ~0.05ms per request
- **Caching**: Improves performance for repeated queries
- **Authentication**: ~0.5ms per request (if enabled)

### When to Use Which

**Use Standard Server when**:
- Building quick prototypes
- Running integration tests
- Embedding in other applications
- Performance-critical scenarios where every microsecond counts

**Use Enterprise Server when**:
- Running in production
- Need observability and monitoring
- Security requirements (TLS, auth)
- Multiple backends (Oracle, DuckDB, etc.)
- Need caching and connection pooling

## Development Guidelines

### Adding New Features

1. **Core Protocol Features**: Add to `pkg/server/flight_sql.go`
   - Pure protocol implementation
   - No dependencies on enterprise features
   - Thoroughly tested

2. **Enterprise Features**: Add to `cmd/server/main.go`
   - Wrap core functionality with enterprise logic
   - Add metrics, logging, caching as needed
   - Maintain backward compatibility

### Testing Strategy

- **Unit Tests**: Test core server independently
- **Integration Tests**: Test enterprise server with real backends
- **E2E Tests**: Test full stack with client libraries

## Migration Guide

If you're currently using the standard server and want to upgrade to the enterprise server:

1. Update your imports from `pkg/server` to use the `cmd/server` binary
2. Add a configuration file (see `config/oracle_config.yaml` for example)
3. Configure metrics endpoint (optional)
4. Configure authentication (optional)
5. Rebuild and deploy

## Troubleshooting

### "Method not implemented" errors

If you see errors like `GetFlightInfoTables not implemented`:
1. Check you're using the latest version with all metadata methods
2. Rebuild the server: `make build`
3. Restart the service

### Performance issues

If the enterprise server is too slow:
1. Disable authentication if not needed
2. Reduce logging level to `warn` or `error`
3. Adjust cache size in configuration
4. Consider using the standard server for that specific use case

## References

- [Flight SQL Protocol Specification](https://arrow.apache.org/docs/format/FlightSql.html)
- [Apache Arrow Flight](https://arrow.apache.org/docs/format/Flight.html)
- [Porter Configuration Guide](../config/oracle_config.yaml)
- [Deployment Instructions](../DEPLOY_INSTRUCTIONS.md)

