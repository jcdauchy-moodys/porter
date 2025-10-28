package converter

import (
	"database/sql"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeConverter(t *testing.T) {
	logger := zerolog.New(zerolog.NewTestWriter(t))
	tc := New(logger)

	t.Run("DuckDBToArrowType", func(t *testing.T) {
		tests := []struct {
			name     string
			duckType string
			want     arrow.DataType
			wantErr  bool
		}{
			{
				name:     "tinyint",
				duckType: "tinyint",
				want:     arrow.PrimitiveTypes.Int8,
			},
			{
				name:     "smallint",
				duckType: "smallint",
				want:     arrow.PrimitiveTypes.Int16,
			},
			{
				name:     "integer",
				duckType: "integer",
				want:     arrow.PrimitiveTypes.Int32,
			},
			{
				name:     "bigint",
				duckType: "bigint",
				want:     arrow.PrimitiveTypes.Int64,
			},
			{
				name:     "real",
				duckType: "real",
				want:     arrow.PrimitiveTypes.Float32,
			},
			{
				name:     "double",
				duckType: "double",
				want:     arrow.PrimitiveTypes.Float64,
			},
			{
				name:     "boolean",
				duckType: "boolean",
				want:     arrow.FixedWidthTypes.Boolean,
			},
			{
				name:     "varchar",
				duckType: "varchar",
				want:     arrow.BinaryTypes.String,
			},
			{
				name:     "decimal",
				duckType: "decimal(18,2)",
				want:     &arrow.Decimal128Type{Precision: 18, Scale: 2},
			},
			{
				name:     "numeric",
				duckType: "numeric(10,4)",
				want:     &arrow.Decimal128Type{Precision: 10, Scale: 4},
			},
			{
				name:     "Oracle NUMBER without precision",
				duckType: "NUMBER",
				want:     arrow.PrimitiveTypes.Float64,
			},
			{
				name:     "Oracle number lowercase",
				duckType: "number",
				want:     arrow.PrimitiveTypes.Float64,
			},
			{
				name:     "Oracle NUMBER with precision",
				duckType: "NUMBER(10,2)",
				want:     &arrow.Decimal128Type{Precision: 10, Scale: 2},
			},
			{
				name:     "Oracle number with precision lowercase",
				duckType: "number(15,3)",
				want:     &arrow.Decimal128Type{Precision: 15, Scale: 3},
			},
			{
				name:     "Oracle BINARY_FLOAT",
				duckType: "BINARY_FLOAT",
				want:     arrow.PrimitiveTypes.Float32,
			},
			{
				name:     "Oracle BINARY_DOUBLE",
				duckType: "BINARY_DOUBLE",
				want:     arrow.PrimitiveTypes.Float64,
			},
			{
				name:     "Oracle VARCHAR2",
				duckType: "VARCHAR2",
				want:     arrow.BinaryTypes.String,
			},
			{
				name:     "Oracle NVARCHAR2",
				duckType: "NVARCHAR2",
				want:     arrow.BinaryTypes.String,
			},
			{
				name:     "Oracle CLOB",
				duckType: "CLOB",
				want:     arrow.BinaryTypes.String,
			},
			{
				name:     "Oracle NCLOB",
				duckType: "NCLOB",
				want:     arrow.BinaryTypes.String,
			},
			{
				name:     "Oracle RAW",
				duckType: "RAW",
				want:     arrow.BinaryTypes.Binary,
			},
			{
				name:     "Oracle LONG RAW",
				duckType: "LONG RAW",
				want:     arrow.BinaryTypes.Binary,
			},
			{
				name:     "Oracle TIMESTAMP WITH TIME ZONE",
				duckType: "TIMESTAMP WITH TIME ZONE",
				want:     arrow.FixedWidthTypes.Timestamp_us,
			},
			{
				name:     "Oracle TIMESTAMP WITH LOCAL TIME ZONE",
				duckType: "TIMESTAMP WITH LOCAL TIME ZONE",
				want:     arrow.FixedWidthTypes.Timestamp_us,
			},
			{
				name:     "Oracle INTERVAL YEAR TO MONTH",
				duckType: "INTERVAL YEAR TO MONTH",
				want:     arrow.FixedWidthTypes.MonthInterval,
			},
			{
				name:     "Oracle INTERVAL DAY TO SECOND",
				duckType: "INTERVAL DAY TO SECOND",
				want:     arrow.FixedWidthTypes.Duration_ns,
			},
			{
				name:     "Oracle ROWID",
				duckType: "ROWID",
				want:     arrow.BinaryTypes.String,
			},
			{
				name:     "Oracle UROWID",
				duckType: "UROWID",
				want:     arrow.BinaryTypes.String,
			},
			{
				name:     "invalid type",
				duckType: "invalid_type",
				wantErr:  true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := tc.DuckDBToArrowType(tt.duckType)
				if tt.wantErr {
					assert.Error(t, err)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("ArrowToDuckDBType", func(t *testing.T) {
		tests := []struct {
			name      string
			arrowType arrow.DataType
			want      string
			wantErr   bool
		}{
			{
				name:      "int8",
				arrowType: arrow.PrimitiveTypes.Int8,
				want:      "TINYINT",
			},
			{
				name:      "int16",
				arrowType: arrow.PrimitiveTypes.Int16,
				want:      "SMALLINT",
			},
			{
				name:      "int32",
				arrowType: arrow.PrimitiveTypes.Int32,
				want:      "INTEGER",
			},
			{
				name:      "int64",
				arrowType: arrow.PrimitiveTypes.Int64,
				want:      "BIGINT",
			},
			{
				name:      "float32",
				arrowType: arrow.PrimitiveTypes.Float32,
				want:      "FLOAT",
			},
			{
				name:      "float64",
				arrowType: arrow.PrimitiveTypes.Float64,
				want:      "DOUBLE",
			},
			{
				name:      "boolean",
				arrowType: arrow.FixedWidthTypes.Boolean,
				want:      "BOOLEAN",
			},
			{
				name:      "string",
				arrowType: arrow.BinaryTypes.String,
				want:      "VARCHAR",
			},
			{
				name:      "decimal",
				arrowType: &arrow.Decimal128Type{Precision: 18, Scale: 2},
				want:      "DECIMAL(18,2)",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := tc.ArrowToDuckDBType(tt.arrowType)
				if tt.wantErr {
					assert.Error(t, err)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("GetSQLType", func(t *testing.T) {
		tests := []struct {
			name     string
			duckType string
			want     int32
		}{
			{
				name:     "tinyint",
				duckType: "tinyint",
				want:     int32(java_sql_Types_TINYINT),
			},
			{
				name:     "smallint",
				duckType: "smallint",
				want:     int32(java_sql_Types_SMALLINT),
			},
			{
				name:     "integer",
				duckType: "integer",
				want:     int32(java_sql_Types_INTEGER),
			},
			{
				name:     "bigint",
				duckType: "bigint",
				want:     int32(java_sql_Types_BIGINT),
			},
			{
				name:     "real",
				duckType: "real",
				want:     int32(java_sql_Types_REAL),
			},
			{
				name:     "float",
				duckType: "float",
				want:     int32(java_sql_Types_FLOAT),
			},
			{
				name:     "double",
				duckType: "double",
				want:     int32(java_sql_Types_DOUBLE),
			},
			{
				name:     "decimal",
				duckType: "decimal",
				want:     int32(java_sql_Types_DECIMAL),
			},
			{
				name:     "numeric",
				duckType: "numeric",
				want:     int32(java_sql_Types_NUMERIC),
			},
			{
				name:     "Oracle number",
				duckType: "number",
				want:     int32(java_sql_Types_NUMERIC),
			},
			{
				name:     "Oracle NUMBER uppercase",
				duckType: "NUMBER",
				want:     int32(java_sql_Types_NUMERIC),
			},
			{
				name:     "Oracle binary_float",
				duckType: "binary_float",
				want:     int32(java_sql_Types_FLOAT),
			},
			{
				name:     "Oracle binary_double",
				duckType: "binary_double",
				want:     int32(java_sql_Types_DOUBLE),
			},
			{
				name:     "Oracle varchar2",
				duckType: "varchar2",
				want:     int32(java_sql_Types_VARCHAR),
			},
			{
				name:     "Oracle nvarchar2",
				duckType: "nvarchar2",
				want:     int32(java_sql_Types_NVARCHAR),
			},
			{
				name:     "Oracle clob",
				duckType: "clob",
				want:     int32(java_sql_Types_CLOB),
			},
			{
				name:     "Oracle nclob",
				duckType: "nclob",
				want:     int32(java_sql_Types_NCLOB),
			},
			{
				name:     "Oracle raw",
				duckType: "raw",
				want:     int32(java_sql_Types_VARBINARY),
			},
			{
				name:     "Oracle long raw",
				duckType: "long raw",
				want:     int32(java_sql_Types_LONGVARBINARY),
			},
			{
				name:     "Oracle timestamp with time zone",
				duckType: "timestamp with time zone",
				want:     int32(java_sql_Types_TIMESTAMP_WITH_TIMEZONE),
			},
			{
				name:     "Oracle rowid",
				duckType: "rowid",
				want:     int32(java_sql_Types_ROWID),
			},
			{
				name:     "Oracle urowid",
				duckType: "urowid",
				want:     int32(java_sql_Types_ROWID),
			},
			{
				name:     "boolean",
				duckType: "boolean",
				want:     int32(java_sql_Types_BOOLEAN),
			},
			{
				name:     "varchar",
				duckType: "varchar",
				want:     int32(java_sql_Types_VARCHAR),
			},
			{
				name:     "text",
				duckType: "text",
				want:     int32(java_sql_Types_VARCHAR),
			},
			{
				name:     "blob",
				duckType: "blob",
				want:     int32(java_sql_Types_BLOB),
			},
			{
				name:     "date",
				duckType: "date",
				want:     int32(java_sql_Types_DATE),
			},
			{
				name:     "time",
				duckType: "time",
				want:     int32(java_sql_Types_TIME),
			},
			{
				name:     "timestamp",
				duckType: "timestamp",
				want:     int32(java_sql_Types_TIMESTAMP),
			},
			{
				name:     "unknown type",
				duckType: "unknown",
				want:     int32(java_sql_Types_VARCHAR), // Default to VARCHAR
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := tc.GetSQLType(tt.duckType)
				assert.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("DuckDBToArrowValue", func(t *testing.T) {
		tests := []struct {
			name      string
			value     interface{}
			arrowType arrow.DataType
			want      interface{}
			wantErr   bool
		}{
			{
				name:      "null value",
				value:     nil,
				arrowType: arrow.PrimitiveTypes.Int32,
				want:      nil,
			},
			{
				name:      "sql.NullInt32 valid",
				value:     sql.NullInt32{Int32: 42, Valid: true},
				arrowType: arrow.PrimitiveTypes.Int32,
				want:      int32(42),
			},
			{
				name:      "sql.NullInt32 invalid",
				value:     sql.NullInt32{Valid: false},
				arrowType: arrow.PrimitiveTypes.Int32,
				want:      nil,
			},
			{
				name:      "sql.NullString valid",
				value:     sql.NullString{String: "test", Valid: true},
				arrowType: arrow.BinaryTypes.String,
				want:      "test",
			},
			{
				name:      "sql.NullString invalid",
				value:     sql.NullString{Valid: false},
				arrowType: arrow.BinaryTypes.String,
				want:      nil,
			},
			{
				name:      "direct int32",
				value:     int32(42),
				arrowType: arrow.PrimitiveTypes.Int32,
				want:      int32(42),
			},
			{
				name:      "direct string",
				value:     "test",
				arrowType: arrow.BinaryTypes.String,
				want:      "test",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := tc.DuckDBToArrowValue(tt.value, tt.arrowType)
				if tt.wantErr {
					assert.Error(t, err)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			})
		}
	})

	t.Run("ArrowToDuckDBValue", func(t *testing.T) {
		tests := []struct {
			name     string
			value    interface{}
			duckType string
			want     interface{}
			wantErr  bool
		}{
			{
				name:     "null value",
				value:    nil,
				duckType: "integer",
				want:     nil,
			},
			{
				name:     "int32",
				value:    int32(42),
				duckType: "integer",
				want:     int32(42),
			},
			{
				name:     "string",
				value:    "test",
				duckType: "varchar",
				want:     "test",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := tc.ArrowToDuckDBValue(tt.value, tt.duckType)
				if tt.wantErr {
					assert.Error(t, err)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			})
		}
	})
}
