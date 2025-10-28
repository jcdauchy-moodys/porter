// Package converter provides type conversion between DuckDB and Apache Arrow.
package converter

import (
	"database/sql"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/rs/zerolog"

	"github.com/TFMV/porter/pkg/errors"
)

// TypeConverter handles type conversions between DuckDB and Arrow.
type TypeConverter interface {
	// DuckDB to Arrow conversions
	DuckDBToArrowType(duckdbType string) (arrow.DataType, error)
	DuckDBToArrowValue(value interface{}, arrowType arrow.DataType) (interface{}, error)

	// Arrow to DuckDB conversions
	ArrowToDuckDBType(arrowType arrow.DataType) (string, error)
	ArrowToDuckDBValue(value interface{}, duckdbType string) (interface{}, error)

	// SQL type mappings
	GetSQLType(duckdbType string) int32
	GetArrowFieldFromColumn(col *sql.ColumnType) (arrow.Field, error)

	// Schema conversion
	ConvertToArrowSchema(cols []*sql.ColumnType) (*arrow.Schema, error)
}

type typeConverter struct {
	typeMap    map[string]arrow.DataType
	reverseMap map[arrow.Type]string
	sqlMap     map[string]int32
	logger     zerolog.Logger
}

// New creates a new type converter.
func New(logger zerolog.Logger) TypeConverter {
	tc := &typeConverter{
		typeMap:    initializeTypeMap(),
		reverseMap: initializeReverseMap(),
		sqlMap:     initializeSQLMap(),
		logger:     logger,
	}

	return tc
}

// ConvertDuckDBTypeToArrow converts a DuckDB type string to an Apache Arrow DataType.
func ConvertDuckDBTypeToArrow(duckdbType string) (arrow.DataType, error) {
	// Handle decimal, numeric, or number types (Oracle uses NUMBER)
	lowerType := strings.ToLower(duckdbType)
	if strings.HasPrefix(lowerType, "decimal") || strings.HasPrefix(lowerType, "numeric") || strings.HasPrefix(lowerType, "number") {
		// Default precision and scale
		precision := int32(38)
		scale := int32(4)

		// Regular expression to match decimal(p,s), numeric(p,s), or number(p,s)
		// Example: decimal(18,2), numeric(10,3), or NUMBER(10,2)
		re := regexp.MustCompile(`^(decimal|numeric|number)\((\d+),(\d+)\)$`)
		matches := re.FindStringSubmatch(lowerType)

		if len(matches) == 4 {
			// Parse precision
			p, err := strconv.ParseInt(matches[2], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid precision in %s: %w", duckdbType, err)
			}
			// Parse scale
			s, err := strconv.ParseInt(matches[3], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid scale in %s: %w", duckdbType, err)
			}

			// Validate precision and scale
			if p < 1 || p > 38 {
				return nil, fmt.Errorf("precision %d out of range (1-38) for %s", p, duckdbType)
			}
			if s < 0 || s > p {
				return nil, fmt.Errorf("scale %d out of range (0-%d) for %s", s, p, duckdbType)
			}

			precision = int32(p)
			scale = int32(s)
		} else if len(matches) != 0 {
			// If the format is invalid but starts with decimal/numeric
			return nil, fmt.Errorf("invalid decimal/numeric format: %s", duckdbType)
		}

		// Return Decimal128Type with parsed or default precision and scale
		return &arrow.Decimal128Type{Precision: precision, Scale: scale}, nil
	}

	// Handle other types (placeholder for additional type handling)
	return nil, fmt.Errorf("unsupported DuckDB type: %s", duckdbType)
}

// ArrowToDuckDBType converts an Arrow type to a DuckDB type.
func (tc *typeConverter) ArrowToDuckDBType(arrowType arrow.DataType) (string, error) {
	if duckdbType, ok := tc.reverseMap[arrowType.ID()]; ok {
		return duckdbType, nil
	}

	// Handle special cases
	switch arrowType.ID() {
	case arrow.DECIMAL, arrow.DECIMAL256:
		decimalType := arrowType.(arrow.DecimalType)
		return fmt.Sprintf("DECIMAL(%d,%d)", decimalType.GetPrecision(), decimalType.GetScale()), nil
	case arrow.FIXED_SIZE_BINARY:
		fixedType := arrowType.(*arrow.FixedSizeBinaryType)
		return fmt.Sprintf("BLOB(%d)", fixedType.ByteWidth), nil
	case arrow.LIST:
		elemType, err := tc.ArrowToDuckDBType(arrowType.(*arrow.ListType).Elem())
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s[]", elemType), nil
	case arrow.STRUCT:
		// Handle struct types
		return "STRUCT", nil
	default:
		return "", errors.New(errors.CodeInternal, fmt.Sprintf("unsupported Arrow type: %s", arrowType))
	}
}

// GetSQLType returns the SQL type code for a DuckDB type.
func (tc *typeConverter) GetSQLType(duckdbType string) int32 {
	duckdbType = strings.ToLower(strings.TrimSpace(duckdbType))

	if sqlType, ok := tc.sqlMap[duckdbType]; ok {
		return sqlType
	}

	// Default to VARCHAR for unknown types
	return int32(java_sql_Types_VARCHAR)
}

// GetArrowFieldFromColumn converts a SQL column to an Arrow field.
func (tc *typeConverter) GetArrowFieldFromColumn(col *sql.ColumnType) (arrow.Field, error) {
	// Get Arrow type
	arrowType, err := tc.getArrowTypeFromColumnType(col)
	if err != nil {
		return arrow.Field{}, err
	}

	// Build metadata
	metadata := tc.buildColumnMetadata(col)

	// Get nullability
	nullable, _ := col.Nullable()

	// Create field
	field := arrow.Field{
		Name:     col.Name(),
		Type:     arrowType,
		Nullable: nullable,
		Metadata: metadata,
	}

	return field, nil
}

// ConvertToArrowSchema converts SQL column types to an Arrow schema.
func (tc *typeConverter) ConvertToArrowSchema(cols []*sql.ColumnType) (*arrow.Schema, error) {
	fields := make([]arrow.Field, len(cols))

	for i, col := range cols {
		field, err := tc.GetArrowFieldFromColumn(col)
		if err != nil {
			return nil, errors.Wrapf(err, errors.CodeInternal, "failed to convert column %d", i)
		}
		fields[i] = field
	}

	return arrow.NewSchema(fields, nil), nil
}

// getArrowTypeFromColumnType determines Arrow type from SQL column type.
func (tc *typeConverter) getArrowTypeFromColumnType(col *sql.ColumnType) (arrow.DataType, error) {
	// First try database type name
	dbType := col.DatabaseTypeName()
	if dbType != "" {
		arrowType, err := tc.DuckDBToArrowType(dbType)
		if err == nil {
			return arrowType, nil
		}

		// If DuckDBToArrowType failed, try to infer from Oracle internal type names
		// The go-ora driver sometimes returns internal Oracle type constants like "timestampdty"
		inferredType := tc.inferOracleInternalType(dbType)
		if inferredType != nil {
			tc.logger.Debug().
				Str("db_type", dbType).
				Str("inferred_type", inferredType.String()).
				Msg("Inferred Oracle internal type")
			return inferredType, nil
		}

		// If we still can't convert, log the error but continue to scan type fallback
		tc.logger.Warn().
			Str("db_type", dbType).
			Err(err).
			Msg("Failed to convert database type, falling back to scan type")
	}

	// Fall back to scan type
	scanType := col.ScanType()
	if scanType == nil {
		// Default to string if we can't determine type
		return arrow.BinaryTypes.String, nil
	}

	// Map Go types to Arrow types
	switch scanType.Kind() {
	case reflect.Bool:
		return arrow.FixedWidthTypes.Boolean, nil
	case reflect.Int8:
		return arrow.PrimitiveTypes.Int8, nil
	case reflect.Int16:
		return arrow.PrimitiveTypes.Int16, nil
	case reflect.Int32:
		return arrow.PrimitiveTypes.Int32, nil
	case reflect.Int, reflect.Int64:
		return arrow.PrimitiveTypes.Int64, nil
	case reflect.Uint8:
		return arrow.PrimitiveTypes.Uint8, nil
	case reflect.Uint16:
		return arrow.PrimitiveTypes.Uint16, nil
	case reflect.Uint32:
		return arrow.PrimitiveTypes.Uint32, nil
	case reflect.Uint, reflect.Uint64:
		return arrow.PrimitiveTypes.Uint64, nil
	case reflect.Float32:
		return arrow.PrimitiveTypes.Float32, nil
	case reflect.Float64:
		return arrow.PrimitiveTypes.Float64, nil
	case reflect.String:
		return arrow.BinaryTypes.String, nil
	case reflect.Slice:
		if scanType.Elem().Kind() == reflect.Uint8 {
			return arrow.BinaryTypes.Binary, nil
		}
		// Default to string for other slices
		return arrow.BinaryTypes.String, nil
	default:
		// Default to string for unknown types
		return arrow.BinaryTypes.String, nil
	}
}

// inferOracleInternalType attempts to infer Arrow type from Oracle internal type names.
// The go-ora driver sometimes returns internal Oracle type constants that end with "dty" (data type).
// Examples: "timestampdty", "numberdty", "stringdty", etc.
func (tc *typeConverter) inferOracleInternalType(dbType string) arrow.DataType {
	dbTypeLower := strings.ToLower(dbType)

	// Check for common Oracle internal type patterns
	// Order matters - check more specific patterns first
	switch {
	// Date/Time types - check timestamp before date since "timestamp" contains "time"
	case strings.Contains(dbTypeLower, "timestamp"):
		return arrow.FixedWidthTypes.Timestamp_us
	case strings.Contains(dbTypeLower, "interval"):
		return arrow.FixedWidthTypes.MonthDayNanoInterval
	case strings.HasPrefix(dbTypeLower, "date") || strings.Contains(dbTypeLower, "datedty"):
		// Match "date" at start or "datedty" pattern
		return arrow.FixedWidthTypes.Date32
	case strings.Contains(dbTypeLower, "time") && !strings.Contains(dbTypeLower, "timestamp"):
		// Match "time" but not if it's part of "timestamp"
		return arrow.FixedWidthTypes.Time64us

	// Numeric types - check before int since numbers might contain "int"
	case strings.Contains(dbTypeLower, "number") || strings.Contains(dbTypeLower, "numeric"):
		return arrow.PrimitiveTypes.Float64
	case strings.Contains(dbTypeLower, "float") || strings.Contains(dbTypeLower, "double"):
		return arrow.PrimitiveTypes.Float64
	case strings.Contains(dbTypeLower, "int") && !strings.Contains(dbTypeLower, "interval"):
		// Match "int" but not "interval"
		return arrow.PrimitiveTypes.Int64

	// String types
	case strings.Contains(dbTypeLower, "clob"):
		return arrow.BinaryTypes.String
	case strings.Contains(dbTypeLower, "string") || strings.Contains(dbTypeLower, "char") ||
		strings.Contains(dbTypeLower, "varchar"):
		return arrow.BinaryTypes.String

	// Binary types
	case strings.Contains(dbTypeLower, "blob") || strings.Contains(dbTypeLower, "binary") ||
		strings.Contains(dbTypeLower, "raw"):
		return arrow.BinaryTypes.Binary

	// Boolean and special types
	case strings.Contains(dbTypeLower, "bool"):
		return arrow.FixedWidthTypes.Boolean
	case strings.Contains(dbTypeLower, "rowid"):
		return arrow.BinaryTypes.String

	default:
		return nil
	}
}

// buildColumnMetadata builds Arrow metadata for a SQL column.
func (tc *typeConverter) buildColumnMetadata(col *sql.ColumnType) arrow.Metadata {
	keys := []string{}
	values := []string{}

	// Set database type name
	if dbType := col.DatabaseTypeName(); dbType != "" {
		keys = append(keys, "ARROW:FLIGHT:SQL:TYPE_NAME")
		values = append(values, dbType)
	}

	// Set precision and scale for numeric types
	if precision, scale, ok := col.DecimalSize(); ok {
		keys = append(keys, "ARROW:FLIGHT:SQL:PRECISION")
		values = append(values, fmt.Sprintf("%d", precision))
		keys = append(keys, "ARROW:FLIGHT:SQL:SCALE")
		values = append(values, fmt.Sprintf("%d", scale))
	}

	// Set nullable
	if nullable, ok := col.Nullable(); ok {
		keys = append(keys, "ARROW:FLIGHT:SQL:IS_NULLABLE")
		values = append(values, fmt.Sprintf("%t", nullable))
	}

	return arrow.NewMetadata(keys, values)
}

// DuckDBToArrowValue converts a DuckDB value to an Arrow-compatible value.
func (tc *typeConverter) DuckDBToArrowValue(value interface{}, arrowType arrow.DataType) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	// Handle sql.Null* types
	switch v := value.(type) {
	case sql.NullBool:
		if !v.Valid {
			return nil, nil
		}
		return v.Bool, nil
	case sql.NullByte:
		if !v.Valid {
			return nil, nil
		}
		return v.Byte, nil
	case sql.NullFloat64:
		if !v.Valid {
			return nil, nil
		}
		return v.Float64, nil
	case sql.NullInt16:
		if !v.Valid {
			return nil, nil
		}
		return v.Int16, nil
	case sql.NullInt32:
		if !v.Valid {
			return nil, nil
		}
		return v.Int32, nil
	case sql.NullInt64:
		if !v.Valid {
			return nil, nil
		}
		return v.Int64, nil
	case sql.NullString:
		if !v.Valid {
			return nil, nil
		}
		return v.String, nil
	case sql.NullTime:
		if !v.Valid {
			return nil, nil
		}
		return v.Time, nil
	}

	// Direct conversion for most types
	return value, nil
}

// ArrowToDuckDBValue converts an Arrow value to a DuckDB-compatible value.
func (tc *typeConverter) ArrowToDuckDBValue(value interface{}, duckdbType string) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	// Most values can be passed directly
	return value, nil
}

// initializeTypeMap creates the DuckDB to Arrow type mapping.
func initializeTypeMap() map[string]arrow.DataType {
	return map[string]arrow.DataType{
		// Integer types
		"tinyint":   arrow.PrimitiveTypes.Int8,
		"smallint":  arrow.PrimitiveTypes.Int16,
		"integer":   arrow.PrimitiveTypes.Int32,
		"int":       arrow.PrimitiveTypes.Int32,
		"bigint":    arrow.PrimitiveTypes.Int64,
		"hugeint":   arrow.PrimitiveTypes.Int64, // DuckDB HUGEINT mapped to INT64
		"utinyint":  arrow.PrimitiveTypes.Uint8,
		"usmallint": arrow.PrimitiveTypes.Uint16,
		"uinteger":  arrow.PrimitiveTypes.Uint32,
		"uint":      arrow.PrimitiveTypes.Uint32,
		"ubigint":   arrow.PrimitiveTypes.Uint64,

		// Floating point types
		"real":          arrow.PrimitiveTypes.Float32,
		"float":         arrow.PrimitiveTypes.Float32,
		"double":        arrow.PrimitiveTypes.Float64,
		"binary_float":  arrow.PrimitiveTypes.Float32, // Oracle
		"binary_double": arrow.PrimitiveTypes.Float64, // Oracle

		// Numeric types (Oracle uses NUMBER without precision/scale as FLOAT64)
		"number": arrow.PrimitiveTypes.Float64,

		// Boolean type
		"boolean": arrow.FixedWidthTypes.Boolean,
		"bool":    arrow.FixedWidthTypes.Boolean,

		// String types
		"varchar":   arrow.BinaryTypes.String,
		"varchar2":  arrow.BinaryTypes.String, // Oracle
		"nvarchar":  arrow.BinaryTypes.String,
		"nvarchar2": arrow.BinaryTypes.String, // Oracle
		"char":      arrow.BinaryTypes.String,
		"nchar":     arrow.BinaryTypes.String, // Oracle
		"text":      arrow.BinaryTypes.String,
		"string":    arrow.BinaryTypes.String,
		"clob":      arrow.BinaryTypes.String, // Oracle
		"nclob":     arrow.BinaryTypes.String, // Oracle

		// Binary types
		"blob":      arrow.BinaryTypes.Binary,
		"bytea":     arrow.BinaryTypes.Binary,
		"varbinary": arrow.BinaryTypes.Binary,
		"raw":       arrow.BinaryTypes.Binary, // Oracle
		"long raw":  arrow.BinaryTypes.Binary, // Oracle

		// Date/Time types
		"date":                           arrow.FixedWidthTypes.Date32,
		"time":                           arrow.FixedWidthTypes.Time32s,
		"timestamp":                      arrow.FixedWidthTypes.Timestamp_us,
		"timestamp with time zone":       arrow.FixedWidthTypes.Timestamp_us, // Oracle
		"timestamp with local time zone": arrow.FixedWidthTypes.Timestamp_us, // Oracle
		"interval":                       arrow.FixedWidthTypes.MonthDayNanoInterval,
		"interval year to month":         arrow.FixedWidthTypes.MonthInterval, // Oracle
		"interval day to second":         arrow.FixedWidthTypes.Duration_ns,   // Oracle

		// UUID type
		"uuid": arrow.BinaryTypes.String, // UUID as string for compatibility

		// JSON type
		"json": arrow.BinaryTypes.String, // JSON as string

		// Oracle special types
		"rowid":  arrow.BinaryTypes.String, // Oracle ROWID as string
		"urowid": arrow.BinaryTypes.String, // Oracle UROWID as string
	}
}

// initializeReverseMap creates the Arrow to DuckDB type mapping.
func initializeReverseMap() map[arrow.Type]string {
	return map[arrow.Type]string{
		// Integer types
		arrow.INT8:   "TINYINT",
		arrow.INT16:  "SMALLINT",
		arrow.INT32:  "INTEGER",
		arrow.INT64:  "BIGINT",
		arrow.UINT8:  "UTINYINT",
		arrow.UINT16: "USMALLINT",
		arrow.UINT32: "UINTEGER",
		arrow.UINT64: "UBIGINT",

		// Floating point types
		arrow.FLOAT32: "FLOAT",
		arrow.FLOAT64: "DOUBLE",

		// Boolean type
		arrow.BOOL: "BOOLEAN",

		// String type
		arrow.STRING: "VARCHAR",

		// Binary type
		arrow.BINARY: "BLOB",

		// Date/Time types
		arrow.DATE32:                  "DATE",
		arrow.DATE64:                  "DATE",
		arrow.TIME32:                  "TIME",
		arrow.TIME64:                  "TIME",
		arrow.TIMESTAMP:               "TIMESTAMP",
		arrow.INTERVAL_MONTH_DAY_NANO: "INTERVAL",
	}
}

// initializeSQLMap creates the DuckDB type to SQL type code mapping.
func initializeSQLMap() map[string]int32 {
	return map[string]int32{
		// Integer types
		"tinyint":  int32(java_sql_Types_TINYINT),
		"smallint": int32(java_sql_Types_SMALLINT),
		"integer":  int32(java_sql_Types_INTEGER),
		"bigint":   int32(java_sql_Types_BIGINT),

		// Floating point types
		"real":          int32(java_sql_Types_REAL),
		"float":         int32(java_sql_Types_FLOAT),
		"double":        int32(java_sql_Types_DOUBLE),
		"binary_float":  int32(java_sql_Types_FLOAT),  // Oracle
		"binary_double": int32(java_sql_Types_DOUBLE), // Oracle

		// Numeric/Decimal types
		"decimal": int32(java_sql_Types_DECIMAL),
		"numeric": int32(java_sql_Types_NUMERIC),
		"number":  int32(java_sql_Types_NUMERIC), // Oracle NUMBER type

		// Boolean
		"boolean": int32(java_sql_Types_BOOLEAN),

		// String types
		"varchar":   int32(java_sql_Types_VARCHAR),
		"varchar2":  int32(java_sql_Types_VARCHAR),  // Oracle
		"nvarchar":  int32(java_sql_Types_NVARCHAR), // Oracle
		"nvarchar2": int32(java_sql_Types_NVARCHAR), // Oracle
		"char":      int32(java_sql_Types_CHAR),
		"nchar":     int32(java_sql_Types_NCHAR), // Oracle
		"text":      int32(java_sql_Types_VARCHAR),
		"clob":      int32(java_sql_Types_CLOB),  // Oracle
		"nclob":     int32(java_sql_Types_NCLOB), // Oracle

		// Binary types
		"blob":     int32(java_sql_Types_BLOB),
		"raw":      int32(java_sql_Types_VARBINARY),     // Oracle
		"long raw": int32(java_sql_Types_LONGVARBINARY), // Oracle

		// Date/Time types
		"date":                           int32(java_sql_Types_DATE),
		"time":                           int32(java_sql_Types_TIME),
		"timestamp":                      int32(java_sql_Types_TIMESTAMP),
		"timestamp with time zone":       int32(java_sql_Types_TIMESTAMP_WITH_TIMEZONE), // Oracle
		"timestamp with local time zone": int32(java_sql_Types_TIMESTAMP_WITH_TIMEZONE), // Oracle
		"interval year to month":         int32(java_sql_Types_OTHER),                   // Oracle
		"interval day to second":         int32(java_sql_Types_OTHER),                   // Oracle

		// Special types
		"rowid":  int32(java_sql_Types_ROWID), // Oracle
		"urowid": int32(java_sql_Types_ROWID), // Oracle
	}
}

// Java SQL Types constants (from JDBC spec)
const (
	java_sql_Types_BIT                     = -7
	java_sql_Types_TINYINT                 = -6
	java_sql_Types_SMALLINT                = 5
	java_sql_Types_INTEGER                 = 4
	java_sql_Types_BIGINT                  = -5
	java_sql_Types_FLOAT                   = 6
	java_sql_Types_REAL                    = 7
	java_sql_Types_DOUBLE                  = 8
	java_sql_Types_NUMERIC                 = 2
	java_sql_Types_DECIMAL                 = 3
	java_sql_Types_CHAR                    = 1
	java_sql_Types_VARCHAR                 = 12
	java_sql_Types_LONGVARCHAR             = -1
	java_sql_Types_DATE                    = 91
	java_sql_Types_TIME                    = 92
	java_sql_Types_TIMESTAMP               = 93
	java_sql_Types_BINARY                  = -2
	java_sql_Types_VARBINARY               = -3
	java_sql_Types_LONGVARBINARY           = -4
	java_sql_Types_NULL                    = 0
	java_sql_Types_OTHER                   = 1111
	java_sql_Types_JAVA_OBJECT             = 2000
	java_sql_Types_DISTINCT                = 2001
	java_sql_Types_STRUCT                  = 2002
	java_sql_Types_ARRAY                   = 2003
	java_sql_Types_BLOB                    = 2004
	java_sql_Types_CLOB                    = 2005
	java_sql_Types_REF                     = 2006
	java_sql_Types_DATALINK                = 70
	java_sql_Types_BOOLEAN                 = 16
	java_sql_Types_ROWID                   = -8
	java_sql_Types_NCHAR                   = -15
	java_sql_Types_NVARCHAR                = -9
	java_sql_Types_LONGNVARCHAR            = -16
	java_sql_Types_NCLOB                   = 2011
	java_sql_Types_SQLXML                  = 2009
	java_sql_Types_REF_CURSOR              = 2012
	java_sql_Types_TIME_WITH_TIMEZONE      = 2013
	java_sql_Types_TIMESTAMP_WITH_TIMEZONE = 2014
)

// DuckDBToArrowType converts a DuckDB type string to an Apache Arrow DataType.
func (tc *typeConverter) DuckDBToArrowType(duckdbType string) (arrow.DataType, error) {
	duckdbType = strings.ToLower(strings.TrimSpace(duckdbType))
	if arrowType, ok := tc.typeMap[duckdbType]; ok {
		return arrowType, nil
	}
	return ConvertDuckDBTypeToArrow(duckdbType)
}
