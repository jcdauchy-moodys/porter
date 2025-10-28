# Oracle Backend

Porter supports Oracle Database as a backend, allowing you to query Oracle databases using the Apache Arrow Flight SQL protocol.

## Prerequisites

- Oracle Database 11g or later
- Network connectivity to the Oracle database
- Valid Oracle user credentials with appropriate permissions

## Configuration

### Using Configuration File

Create a configuration file (e.g., `oracle_config.yaml`):

```yaml
backend: oracle
address: "0.0.0.0:32010"
log_level: info
max_connections: 100
connection_timeout: 30s
query_timeout: 5m

oracle:
  host: "localhost"
  port: 1521
  service_name: "ORCL"  # or use 'sid' instead
  user: "myuser"
  password: "mypassword"

# Optional TLS configuration
tls:
  enabled: false
  cert_file: ""
  key_file: ""

# Optional authentication
auth:
  enabled: false
  type: basic
```

### Using Command Line Flags

Start the Porter server with Oracle backend using command-line flags:

```bash
porter serve \
  --backend oracle \
  --address 0.0.0.0:32010 \
  --log-level info
```

> **Note**: Oracle connection parameters are best configured through a configuration file or environment variables for security reasons.

### Using Environment Variables

Set environment variables for Oracle connection:

```bash
export PORTER_BACKEND=oracle
export PORTER_ORACLE_HOST=localhost
export PORTER_ORACLE_PORT=1521
export PORTER_ORACLE_SERVICE_NAME=ORCL
export PORTER_ORACLE_USER=myuser
export PORTER_ORACLE_PASSWORD=mypassword

porter serve
```

## Connection Parameters

### Service Name vs SID

Oracle databases can be connected using either a **Service Name** or a **SID**:

- **Service Name** (recommended): Modern Oracle installations use service names
  ```yaml
  oracle:
    service_name: "ORCL"
  ```

- **SID**: Legacy Oracle installations may use SIDs
  ```yaml
  oracle:
    sid: "ORCL"
  ```

> **Note**: You must specify either `service_name` or `sid`, but not both.

### Connection String Format

The Oracle DSN (Data Source Name) is constructed automatically in the format:

```
oracle://username:password@host:port/servicename
```

Or for SID:

```
oracle://username:password@host:port/sid
```

## Supported Features

### Query Operations

- **SELECT queries**: Full support for SELECT statements
- **DML operations**: INSERT, UPDATE, DELETE statements
- **DDL operations**: CREATE, ALTER, DROP statements (requires appropriate permissions)
- **PL/SQL blocks**: Execute PL/SQL anonymous blocks and stored procedures

### Metadata Discovery

Porter supports Flight SQL metadata discovery for Oracle databases:

- **GetCatalogs**: Returns the Oracle database catalog
- **GetSchemas**: Lists all accessible schemas (users)
- **GetTables**: Lists tables and views
- **GetColumns**: Retrieves column information for tables
- **GetPrimaryKeys**: Retrieves primary key information
- **GetImportedKeys**: Retrieves foreign key references to a table
- **GetExportedKeys**: Retrieves foreign keys from a table
- **GetCrossReference**: Retrieves foreign key relationships between tables

### Transaction Support

Porter provides full transaction support for Oracle:

- **BeginTransaction**: Start a new transaction
- **Commit**: Commit the current transaction
- **Rollback**: Roll back the current transaction
- **Isolation levels**: Supports Oracle's isolation levels
  - READ COMMITTED (default)
  - SERIALIZABLE

### Prepared Statements

Prepared statements are fully supported, allowing:

- Parameter binding
- Query plan caching
- Protection against SQL injection
- Improved performance for repeated queries

## Data Type Mapping

Porter automatically maps Oracle data types to Apache Arrow types:

| Oracle Type | Arrow Type |
|------------|-----------|
| NUMBER | DECIMAL or INT (based on precision) |
| FLOAT, BINARY_FLOAT | FLOAT32 |
| BINARY_DOUBLE | FLOAT64 |
| VARCHAR2, NVARCHAR2, CHAR, NCHAR | STRING |
| CLOB, NCLOB | STRING (large) |
| BLOB, RAW, LONG RAW | BINARY |
| DATE | DATE32 |
| TIMESTAMP | TIMESTAMP |
| TIMESTAMP WITH TIME ZONE | TIMESTAMP (with timezone) |
| TIMESTAMP WITH LOCAL TIME ZONE | TIMESTAMP (local) |
| INTERVAL YEAR TO MONTH | INTERVAL_MONTH |
| INTERVAL DAY TO SECOND | INTERVAL_DAY_TIME |
| ROWID, UROWID | STRING |

## Examples

### Python Client Example

```python
from pyarrow import flight
import pyarrow as pa

# Connect to Porter with Oracle backend
client = flight.FlightClient("grpc://localhost:32010")

# Execute a query
flight_info = client.get_flight_info(
    flight.FlightDescriptor.for_command(
        "SELECT employee_id, first_name, last_name, salary FROM employees WHERE department_id = 10"
    )
)

# Read results
reader = client.do_get(flight_info.endpoints[0].ticket)
table = reader.read_all()

# Convert to pandas DataFrame
df = table.to_pandas()
print(df)
```

### JDBC Client Example

```java
import org.apache.arrow.driver.jdbc.ArrowFlightJdbcDriver;
import java.sql.*;

public class OracleFlightSQLExample {
    public static void main(String[] args) throws SQLException {
        String url = "jdbc:arrow-flight-sql://localhost:32010";
        
        try (Connection conn = DriverManager.getConnection(url)) {
            Statement stmt = conn.createStatement();
            ResultSet rs = stmt.executeQuery(
                "SELECT * FROM employees WHERE hire_date > TO_DATE('2020-01-01', 'YYYY-MM-DD')"
            );
            
            while (rs.next()) {
                System.out.println(rs.getString("first_name") + " " + 
                                 rs.getString("last_name"));
            }
        }
    }
}
```

### Using Prepared Statements

```python
from pyarrow import flight

client = flight.FlightClient("grpc://localhost:32010")

# Create a prepared statement
prepared_stmt = client.prepare(
    "SELECT * FROM employees WHERE department_id = ? AND salary > ?"
)

# Execute with parameters
parameters = pa.record_batch([
    pa.array([10], type=pa.int32()),
    pa.array([50000], type=pa.float64())
], names=['department_id', 'salary'])

flight_info = prepared_stmt.execute(parameters)
reader = client.do_get(flight_info.endpoints[0].ticket)
table = reader.read_all()
```

### Working with Transactions

```python
from pyarrow import flight
from pyarrow.flight import FlightClient

client = FlightClient("grpc://localhost:32010")

# Begin transaction
result = client.do_action(
    flight.Action("BeginTransaction", b"")
)
txn_id = next(result).body.to_pybytes().decode('utf-8')

# Execute queries within transaction
client.do_put(
    flight.FlightDescriptor.for_command(
        f"UPDATE employees SET salary = salary * 1.1 WHERE department_id = 10"
    ),
    schema=None,
    options=flight.FlightCallOptions(transaction=txn_id)
)

# Commit transaction
client.do_action(
    flight.Action("EndTransaction", txn_id.encode('utf-8') + b"\x00")
)
```

## Performance Considerations

### Connection Pooling

Porter automatically manages a connection pool for Oracle databases. Configure pool settings:

```yaml
connection_pool:
  max_open_connections: 25
  max_idle_connections: 5
  conn_max_lifetime: 30m
  conn_max_idle_time: 10m
  health_check_period: 1m
```

### Query Optimization

1. **Use bind parameters**: Prepared statements with parameters are more efficient
2. **Limit result sets**: Use WHERE clauses and LIMIT to reduce data transfer
3. **Index usage**: Ensure appropriate indexes exist on frequently queried columns
4. **Batch operations**: Use batch inserts/updates for multiple rows

### Network Optimization

- Enable compression for large result sets
- Use appropriate batch sizes for streaming
- Consider network latency between Porter and Oracle database

## Troubleshooting

### Connection Issues

**Error: "Failed to connect to Oracle database"**

- Verify Oracle listener is running: `lsnrctl status`
- Check network connectivity: `telnet <host> <port>`
- Verify firewall rules allow traffic on port 1521 (or configured port)
- Check TNS names and service availability

**Error: "ORA-12154: TNS:could not resolve the connect identifier"**

- Verify service name or SID is correct
- Check Oracle TNS configuration
- Ensure ORACLE_HOME is set correctly

### Authentication Issues

**Error: "ORA-01017: invalid username/password"**

- Verify credentials are correct
- Check if user account is locked: `SELECT account_status FROM dba_users WHERE username = 'MYUSER';`
- Verify user has CONNECT privilege

**Error: "ORA-28000: the account is locked"**

- Unlock the account: `ALTER USER myuser ACCOUNT UNLOCK;`

### Permission Issues

**Error: "ORA-00942: table or view does not exist"**

- Verify the table exists in the schema
- Check user has SELECT privilege on the table
- If accessing another schema, use schema.table notation
- Grant privileges: `GRANT SELECT ON schema.table TO myuser;`

### Performance Issues

**Slow query execution:**

1. Check Oracle execution plan: Use `EXPLAIN PLAN FOR` in Porter
2. Verify indexes are being used
3. Monitor Oracle AWR reports
4. Check connection pool settings
5. Review network latency

**Connection pool exhaustion:**

- Increase `max_open_connections` in configuration
- Monitor active connections: Check Porter metrics
- Ensure connections are properly closed

## Security Best Practices

1. **Use strong passwords**: Follow Oracle password complexity requirements
2. **Principle of least privilege**: Grant only necessary permissions
3. **Enable TLS**: Configure TLS for encrypted communication
4. **Rotate credentials**: Regularly update passwords
5. **Audit logging**: Enable Oracle audit logging for sensitive operations
6. **Network security**: Use firewalls and VPNs for database access
7. **Secrets management**: Use environment variables or secrets managers for credentials

## Monitoring and Metrics

Porter exposes Prometheus metrics for Oracle backend:

- `porter_oracle_connections_active`: Number of active Oracle connections
- `porter_oracle_queries_total`: Total number of queries executed
- `porter_oracle_query_duration_seconds`: Query execution time histogram
- `porter_oracle_errors_total`: Total number of errors

Access metrics at: `http://localhost:9090/metrics`

## Limitations

1. **LOB data**: Large CLOB/BLOB data may have size limitations
2. **Complex types**: Some Oracle-specific types may have limited support
3. **PL/SQL debugging**: PL/SQL debugging features are not available
4. **Oracle-specific features**: Some advanced Oracle features may not be fully supported through Flight SQL

## Additional Resources

- [Apache Arrow Flight SQL Specification](https://arrow.apache.org/docs/format/FlightSql.html)
- [Oracle Database Documentation](https://docs.oracle.com/en/database/)
- [Porter GitHub Repository](https://github.com/TFMV/porter)
- [go-ora Driver Documentation](https://github.com/sijms/go-ora)

## Support

For issues, questions, or feature requests related to Oracle backend:

- Open an issue on GitHub
- Check existing documentation
- Review Oracle error codes in Oracle documentation

