# Oracle Backend Implementation Summary

This document provides a summary of the Oracle Database backend implementation for Porter.

## Overview

Porter now supports Oracle Database as a backend, allowing users to query Oracle databases using the Apache Arrow Flight SQL protocol. This implementation provides full Flight SQL functionality including queries, metadata discovery, transactions, and prepared statements.

## Implementation Components

### 1. Driver Integration

- **Driver**: `github.com/sijms/go-ora/v2` - Pure Go Oracle driver
- **Version**: v2.8.22
- **Added to**: `go.mod` and imported in `cmd/server/main.go`

### 2. Repository Implementations

Created Oracle-specific implementations in `pkg/repositories/oracle/`:

#### Query Repository (`query_repository.go`)
- Implements `repositories.QueryRepository` interface
- Supports `ExecuteQuery()` for SELECT statements with Arrow streaming
- Supports `ExecuteUpdate()` for DML operations (INSERT, UPDATE, DELETE)
- Supports `Explain()` using Oracle's `EXPLAIN PLAN FOR` syntax
- Supports `Prepare()` for prepared statements

#### Metadata Repository (`metadata_repository.go`)
- Implements `repositories.MetadataRepository` interface
- Provides metadata discovery capabilities:
  - `GetCatalogs()`: Returns Oracle catalog information
  - `GetSchemas()`: Lists all accessible schemas (Oracle users)
  - `GetTables()`: Lists tables and views
  - `GetColumns()`: Retrieves column metadata
  - `GetPrimaryKeys()`: Returns primary key information
  - `GetImportedKeys()` / `GetExportedKeys()`: Foreign key relationships
  - `GetCrossReference()`: Cross-table foreign key relationships
  - `GetTypeInfo()`: Oracle data type information
  - `GetSQLInfo()`: SQL feature information

#### Transaction Repository (`transaction_repository.go`)
- Implements `repositories.TransactionRepository` interface
- Manages Oracle transactions with proper lifecycle:
  - `Begin()`: Start new transactions with isolation levels
  - `Get()`: Retrieve active transactions
  - `List()`: List all active transactions
  - `Remove()`: Clean up transaction records
- Supports Oracle isolation levels (READ COMMITTED, SERIALIZABLE)

#### Prepared Statement Repository (`prepared_statement_repository.go`)
- Implements `repositories.PreparedStatementRepository` interface
- Manages prepared statement lifecycle:
  - `Store()`: Store prepared statements
  - `Get()`: Retrieve prepared statements by handle
  - `Remove()`: Clean up prepared statements
  - `ExecuteQuery()` / `ExecuteUpdate()`: Execute with parameters
  - `List()`: List prepared statements by transaction

### 3. Configuration Changes

#### Server Configuration (`cmd/server/config/config.go`)
Added:
- `Backend` field to select database type (duckdb, clickhouse, oracle)
- `OracleConfig` struct with Oracle-specific connection parameters:
  - `Host`: Oracle database server hostname/IP
  - `Port`: Oracle listener port (default 1521)
  - `ServiceName`: Oracle service name (recommended)
  - `SID`: Oracle SID (alternative to service name)
  - `User`: Database username
  - `Password`: Database password
- Validation logic for Oracle configuration parameters

#### Command-Line Flags (`cmd/server/main.go`)
Added flags:
- `--backend`: Select backend type
- `--oracle-host`: Oracle hostname
- `--oracle-port`: Oracle port
- `--oracle-service-name`: Oracle service name
- `--oracle-sid`: Oracle SID
- `--oracle-user`: Oracle username
- `--oracle-password`: Oracle password

### 4. Connection Pool Updates (`pkg/infrastructure/pool/connection_pool.go`)

Modified connection pool to support multiple database drivers:
- Added `DriverName` field to `Config` struct
- Updated `New()` function to use configurable driver name
- Modified logging to be database-agnostic
- Maintains backward compatibility with DuckDB

### 5. Server Initialization (`cmd/server/main.go`)

Updated `createEnterpriseServer()` function:
- Builds Oracle DSN from configuration: `oracle://user:password@host:port/servicename`
- Selects appropriate repositories based on `Backend` configuration
- Initializes Oracle repositories when backend is set to "oracle"
- Falls back to DuckDB for other backends

## Connection String Format

Oracle connections use the following DSN format:

```
oracle://username:password@hostname:port/servicename
```

Or with SID:

```
oracle://username:password@hostname:port/sid
```

## Data Type Mapping

Porter automatically maps Oracle data types to Apache Arrow types:

| Oracle Type | Arrow Type | Notes |
|------------|-----------|-------|
| NUMBER | DECIMAL/INT | Based on precision and scale |
| FLOAT, BINARY_FLOAT | FLOAT32 | 32-bit floating point |
| BINARY_DOUBLE | FLOAT64 | 64-bit floating point |
| VARCHAR2, NVARCHAR2, CHAR, NCHAR | STRING | Variable/fixed length strings |
| CLOB, NCLOB | STRING | Large character objects |
| BLOB, RAW, LONG RAW | BINARY | Binary data |
| DATE | DATE32 | Date only |
| TIMESTAMP | TIMESTAMP | Date and time |
| TIMESTAMP WITH TIME ZONE | TIMESTAMP | With timezone |
| TIMESTAMP WITH LOCAL TIME ZONE | TIMESTAMP | Local timezone |
| INTERVAL YEAR TO MONTH | INTERVAL_MONTH | Year-month interval |
| INTERVAL DAY TO SECOND | INTERVAL_DAY_TIME | Day-time interval |
| ROWID, UROWID | STRING | Row identifiers |

## Configuration Files

### Example Oracle Configuration (`config/oracle_config.yaml`)

Provides a complete example configuration for Oracle backend with:
- Oracle connection parameters
- TLS configuration
- Authentication settings
- Connection pool tuning
- Transaction settings
- Cache configuration
- Metrics and health checks

## Documentation

### Oracle Backend Guide (`docs/backends/oracle.md`)

Comprehensive documentation including:
- Prerequisites and requirements
- Configuration methods (YAML, CLI, environment variables)
- Connection parameter details
- Supported features overview
- Data type mapping reference
- Client examples (Python, Java/JDBC)
- Transaction and prepared statement usage
- Performance tuning recommendations
- Troubleshooting guide
- Security best practices
- Monitoring and metrics
- Known limitations

## Usage Examples

### Start Server with Oracle Backend

Using configuration file:
```bash
porter serve --config config/oracle_config.yaml
```

Using command-line flags:
```bash
porter serve \
  --backend oracle \
  --oracle-host localhost \
  --oracle-port 1521 \
  --oracle-service-name ORCL \
  --oracle-user myuser \
  --oracle-password mypassword
```

Using environment variables:
```bash
export PORTER_BACKEND=oracle
export PORTER_ORACLE_HOST=localhost
export PORTER_ORACLE_PORT=1521
export PORTER_ORACLE_SERVICE_NAME=ORCL
export PORTER_ORACLE_USER=myuser
export PORTER_ORACLE_PASSWORD=mypassword
porter serve
```

### Python Client Example

```python
from pyarrow import flight

client = flight.FlightClient("grpc://localhost:32010")

# Execute query
flight_info = client.get_flight_info(
    flight.FlightDescriptor.for_command(
        "SELECT * FROM employees WHERE department_id = 10"
    )
)

reader = client.do_get(flight_info.endpoints[0].ticket)
table = reader.read_all()
df = table.to_pandas()
```

## Testing

To test the Oracle backend implementation:

1. Set up Oracle Database (11g or later)
2. Create a test user with appropriate permissions
3. Update configuration with connection details
4. Start Porter server with Oracle backend
5. Connect using a Flight SQL client (Python, JDBC, etc.)
6. Execute queries and verify results

## Future Enhancements

Potential improvements for the Oracle backend:

1. **Advanced PL/SQL Support**: Better handling of stored procedures and packages
2. **Oracle-specific Optimizations**: Leverage Oracle hints and features
3. **Connection Retry Logic**: Enhanced retry mechanisms for Oracle-specific errors
4. **Advanced Type Support**: Better handling of Oracle-specific types (XMLType, etc.)
5. **RAC Support**: Oracle Real Application Clusters support
6. **Connection Failover**: Automatic failover to standby databases
7. **Performance Monitoring**: Oracle-specific performance metrics
8. **Batch Operations**: Optimized batch insert/update operations

## Dependencies

Key dependencies for Oracle support:

- `github.com/sijms/go-ora/v2`: Pure Go Oracle driver
- `github.com/apache/arrow-go/v18`: Apache Arrow implementation
- `database/sql`: Standard Go database interface

## Security Considerations

1. **Credential Management**: Never commit passwords to version control
2. **Use Environment Variables**: Store sensitive data in environment variables
3. **TLS Encryption**: Enable TLS for encrypted communication
4. **Principle of Least Privilege**: Grant minimal database permissions
5. **Password Rotation**: Regularly update database credentials
6. **Audit Logging**: Enable Oracle audit logging for compliance
7. **Network Security**: Use firewalls and VPNs to secure database access

## Troubleshooting

Common issues and solutions:

1. **Connection Refused**: Verify Oracle listener is running and firewall allows traffic
2. **Invalid Credentials**: Check username/password and account status
3. **Service Name Not Found**: Verify service name or SID is correct
4. **Slow Queries**: Check Oracle execution plans and indexes
5. **Pool Exhaustion**: Increase max_open_connections in configuration

## References

- [Oracle Database Documentation](https://docs.oracle.com/en/database/)
- [Apache Arrow Flight SQL](https://arrow.apache.org/docs/format/FlightSql.html)
- [go-ora Driver](https://github.com/sijms/go-ora)
- [Porter Documentation](https://github.com/TFMV/porter)

## Contributors

This Oracle backend implementation was created to extend Porter's database support beyond analytical databases to include enterprise transactional databases.

## License

This implementation follows the same MIT license as the Porter project.

