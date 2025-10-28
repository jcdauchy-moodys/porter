// Package oracle provides Oracle‑specific repository implementations.
package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/rs/zerolog"

	"github.com/TFMV/porter/pkg/errors"
	"github.com/TFMV/porter/pkg/infrastructure"
	"github.com/TFMV/porter/pkg/infrastructure/converter"
	"github.com/TFMV/porter/pkg/infrastructure/pool"
	"github.com/TFMV/porter/pkg/models"
	"github.com/TFMV/porter/pkg/repositories"
)

// metadataRepository implements repositories.MetadataRepository for Oracle.
type metadataRepository struct {
	pool    pool.ConnectionPool
	sqlInfo *infrastructure.SQLInfoProvider
	log     zerolog.Logger
}

// NewMetadataRepository constructs an Oracle metadata repository.
func NewMetadataRepository(p pool.ConnectionPool, info *infrastructure.SQLInfoProvider, lg zerolog.Logger) repositories.MetadataRepository {
	return &metadataRepository{
		pool:    p,
		sqlInfo: info,
		log:     lg.With().Str("repo", "oracle-metadata").Logger(),
	}
}

//───────────────────────────────────
// public API
//───────────────────────────────────

func (r *metadataRepository) GetCatalogs(context.Context) ([]models.Catalog, error) {
	// Oracle doesn't have a catalog concept in the same way as other DBs
	// We'll return the database name as a single catalog
	return []models.Catalog{{
		Name:        "ORACLE",
		Description: "Oracle Database",
	}}, nil
}

func (r *metadataRepository) GetSchemas(ctx context.Context, catalog, pattern string) ([]models.Schema, error) {
	var sb strings.Builder
	sb.WriteString(`
SELECT DISTINCT username as schema_name
FROM   all_users
WHERE  1=1`)

	args := make([]interface{}, 0, 1)
	if pattern != "" && pattern != "%" {
		sb.WriteString(" AND username LIKE :1")
		args = append(args, strings.ToUpper(pattern))
	}
	sb.WriteString(" ORDER BY username")

	db, err := r.conn(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, r.wrapDBErr(err, sb.String())
	}
	defer rows.Close()

	return scanSchemas(rows, catalog)
}

func (r *metadataRepository) GetTables(ctx context.Context, opt models.GetTablesOptions) ([]models.Table, error) {
	var sb strings.Builder
	sb.WriteString(`
SELECT 
    :1 as table_catalog,
    owner as table_schema, 
    table_name, 
    'BASE TABLE' as table_type
FROM   all_tables
WHERE  1=1`)

	args := []interface{}{"ORACLE"}
	argCount := 2

	if opt.SchemaFilterPattern != nil && !isWild(strPtr(opt.SchemaFilterPattern)) {
		sb.WriteString(fmt.Sprintf(" AND owner LIKE :%d", argCount))
		args = append(args, strings.ToUpper(likeDeref(opt.SchemaFilterPattern)))
		argCount++
	}
	if !isWild(strPtr(opt.TableNameFilterPattern)) {
		sb.WriteString(fmt.Sprintf(" AND table_name LIKE :%d", argCount))
		args = append(args, strings.ToUpper(likeDeref(opt.TableNameFilterPattern)))
		argCount++
	}

	// Add views if table types include VIEW
	hasView := false
	for _, t := range opt.TableTypes {
		if t == "VIEW" {
			hasView = true
			break
		}
	}

	// If specific types are requested and VIEW is included, add UNION for views
	if len(opt.TableTypes) > 0 && hasView {
		sb.WriteString(" UNION ALL ")
		sb.WriteString(`
SELECT 
    :1 as table_catalog,
    owner as table_schema, 
    view_name as table_name, 
    'VIEW' as table_type
FROM   all_views
WHERE  1=1`)

		if opt.SchemaFilterPattern != nil && !isWild(strPtr(opt.SchemaFilterPattern)) {
			sb.WriteString(fmt.Sprintf(" AND owner LIKE :%d", argCount))
			args = append(args, strings.ToUpper(likeDeref(opt.SchemaFilterPattern)))
			argCount++
		}
		if !isWild(strPtr(opt.TableNameFilterPattern)) {
			sb.WriteString(fmt.Sprintf(" AND view_name LIKE :%d", argCount))
			args = append(args, strings.ToUpper(likeDeref(opt.TableNameFilterPattern)))
		}
	}

	sb.WriteString(" ORDER BY table_schema, table_name")

	db, err := r.conn(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, r.wrapDBErr(err, sb.String())
	}
	defer rows.Close()

	return scanTables(rows)
}

func (r *metadataRepository) GetTableTypes(context.Context) ([]string, error) {
	return []string{"BASE TABLE", "VIEW"}, nil
}

func (r *metadataRepository) GetColumns(ctx context.Context, ref models.TableRef) ([]models.Column, error) {
	var sb strings.Builder
	sb.WriteString(`
SELECT 
    :1 as table_catalog,
    owner as table_schema,
    table_name,
    column_name,
    column_id as ordinal_position,
    data_default as column_default,
    CASE WHEN nullable = 'Y' THEN 'YES' ELSE 'NO' END as is_nullable,
    data_type,
    char_length as character_maximum_length,
    data_precision as numeric_precision,
    data_scale as numeric_scale,
    NULL as datetime_precision
FROM   all_tab_columns
WHERE  table_name = :2`)

	args := []interface{}{"ORACLE", strings.ToUpper(ref.Table)}
	argCount := 3

	if s := strPtr(ref.DBSchema); s != "" {
		sb.WriteString(fmt.Sprintf(" AND owner = :%d", argCount))
		args = append(args, strings.ToUpper(s))
		argCount++
	}
	sb.WriteString(" ORDER BY column_id")

	db, err := r.conn(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, r.wrapDBErr(err, sb.String())
	}
	defer rows.Close()

	return scanColumns(rows)
}

func (r *metadataRepository) GetTableSchema(ctx context.Context, ref models.TableRef) (*arrow.Schema, error) {
	var sb strings.Builder
	sb.WriteString("SELECT * FROM ")
	if s := strPtr(ref.DBSchema); s != "" {
		sb.WriteString(quoteIdentifier(strings.ToUpper(s)))
		sb.WriteRune('.')
	}
	sb.WriteString(quoteIdentifier(strings.ToUpper(ref.Table)))
	sb.WriteString(" WHERE ROWNUM <= 0")

	db, err := r.conn(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, sb.String())
	if err != nil {
		return nil, r.wrapDBErr(err, sb.String())
	}
	defer rows.Close()

	cols, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}
	tc := converter.New(r.log)
	schema, err := tc.ConvertToArrowSchema(cols)
	if err != nil {
		return nil, err
	}
	return schema, nil
}

func (r *metadataRepository) GetPrimaryKeys(ctx context.Context, ref models.TableRef) ([]models.Key, error) {
	const q = `
SELECT 
    c.owner,
    c.table_name,
    cc.column_name,
    cc.position
FROM   all_constraints c
JOIN   all_cons_columns cc ON c.constraint_name = cc.constraint_name 
                           AND c.owner = cc.owner
WHERE  c.constraint_type = 'P'
AND    c.table_name = :1`

	args := []interface{}{strings.ToUpper(ref.Table)}
	argCount := 2

	db, err := r.conn(ctx)
	if err != nil {
		return nil, err
	}

	query := q
	if s := strPtr(ref.DBSchema); s != "" {
		query += fmt.Sprintf(" AND c.owner = :%d", argCount)
		args = append(args, strings.ToUpper(s))
		argCount++
	}
	query += " ORDER BY cc.position"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, r.wrapDBErr(err, query)
	}
	defer rows.Close()

	out := make([]models.Key, 0)
	catalog := "ORACLE"
	schema := strPtr(ref.DBSchema)

	for rows.Next() {
		var owner, table, name string
		var position int32
		if err := rows.Scan(&owner, &table, &name, &position); err != nil {
			return nil, err
		}
		out = append(out, models.Key{
			CatalogName: catalog,
			SchemaName:  schema,
			TableName:   ref.Table,
			ColumnName:  name,
			KeySequence: position,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (r *metadataRepository) GetImportedKeys(ctx context.Context, ref models.TableRef) ([]models.ForeignKey, error) {
	const q = `
SELECT 
    pc.owner as pk_owner,
    pc.table_name as pk_table,
    pcc.column_name as pk_column,
    fc.owner as fk_owner,
    fc.table_name as fk_table,
    fcc.column_name as fk_column,
    fcc.position,
    fc.constraint_name as fk_name,
    pc.constraint_name as pk_name,
    fc.delete_rule
FROM   all_constraints fc
JOIN   all_cons_columns fcc ON fc.constraint_name = fcc.constraint_name 
                            AND fc.owner = fcc.owner
JOIN   all_constraints pc ON fc.r_constraint_name = pc.constraint_name 
                          AND fc.r_owner = pc.owner
JOIN   all_cons_columns pcc ON pc.constraint_name = pcc.constraint_name 
                            AND pc.owner = pcc.owner 
                            AND fcc.position = pcc.position
WHERE  fc.constraint_type = 'R'
AND    fc.table_name = :1`

	args := []interface{}{strings.ToUpper(ref.Table)}
	argCount := 2

	db, err := r.conn(ctx)
	if err != nil {
		return nil, err
	}

	query := q
	if s := strPtr(ref.DBSchema); s != "" {
		query += fmt.Sprintf(" AND fc.owner = :%d", argCount)
		args = append(args, strings.ToUpper(s))
		argCount++
	}
	query += " ORDER BY fc.constraint_name, fcc.position"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, r.wrapDBErr(err, query)
	}
	defer rows.Close()

	var fks []models.ForeignKey
	catalog := "ORACLE"

	for rows.Next() {
		var pkOwner, pkTable, pkColumn, fkOwner, fkTable, fkColumn, fkName, pkName, deleteRule string
		var position int32
		if err := rows.Scan(&pkOwner, &pkTable, &pkColumn, &fkOwner, &fkTable, &fkColumn, &position, &fkName, &pkName, &deleteRule); err != nil {
			return nil, err
		}

		fks = append(fks, models.ForeignKey{
			PKCatalogName: catalog,
			PKSchemaName:  pkOwner,
			PKTableName:   pkTable,
			PKColumnName:  pkColumn,
			FKCatalogName: catalog,
			FKSchemaName:  fkOwner,
			FKTableName:   fkTable,
			FKColumnName:  fkColumn,
			KeySequence:   position,
			PKKeyName:     pkName,
			FKKeyName:     fkName,
			UpdateRule:    models.FKRuleNoAction, // Oracle doesn't support UPDATE rules
			DeleteRule:    toFKRule(deleteRule),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return fks, nil
}

func (r *metadataRepository) GetExportedKeys(ctx context.Context, ref models.TableRef) ([]models.ForeignKey, error) {
	const q = `
SELECT 
    pc.owner as pk_owner,
    pc.table_name as pk_table,
    pcc.column_name as pk_column,
    fc.owner as fk_owner,
    fc.table_name as fk_table,
    fcc.column_name as fk_column,
    fcc.position,
    fc.constraint_name as fk_name,
    pc.constraint_name as pk_name,
    fc.delete_rule
FROM   all_constraints pc
JOIN   all_cons_columns pcc ON pc.constraint_name = pcc.constraint_name 
                            AND pc.owner = pcc.owner
JOIN   all_constraints fc ON pc.constraint_name = fc.r_constraint_name 
                          AND pc.owner = fc.r_owner
JOIN   all_cons_columns fcc ON fc.constraint_name = fcc.constraint_name 
                            AND fc.owner = fcc.owner 
                            AND pcc.position = fcc.position
WHERE  pc.constraint_type = 'P'
AND    fc.constraint_type = 'R'
AND    pc.table_name = :1`

	args := []interface{}{strings.ToUpper(ref.Table)}
	argCount := 2

	db, err := r.conn(ctx)
	if err != nil {
		return nil, err
	}

	query := q
	if s := strPtr(ref.DBSchema); s != "" {
		query += fmt.Sprintf(" AND pc.owner = :%d", argCount)
		args = append(args, strings.ToUpper(s))
		argCount++
	}
	query += " ORDER BY fc.constraint_name, fcc.position"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, r.wrapDBErr(err, query)
	}
	defer rows.Close()

	var fks []models.ForeignKey
	catalog := "ORACLE"

	for rows.Next() {
		var pkOwner, pkTable, pkColumn, fkOwner, fkTable, fkColumn, fkName, pkName, deleteRule string
		var position int32
		if err := rows.Scan(&pkOwner, &pkTable, &pkColumn, &fkOwner, &fkTable, &fkColumn, &position, &fkName, &pkName, &deleteRule); err != nil {
			return nil, err
		}

		fks = append(fks, models.ForeignKey{
			PKCatalogName: catalog,
			PKSchemaName:  pkOwner,
			PKTableName:   pkTable,
			PKColumnName:  pkColumn,
			FKCatalogName: catalog,
			FKSchemaName:  fkOwner,
			FKTableName:   fkTable,
			FKColumnName:  fkColumn,
			KeySequence:   position,
			PKKeyName:     pkName,
			FKKeyName:     fkName,
			UpdateRule:    models.FKRuleNoAction,
			DeleteRule:    toFKRule(deleteRule),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return fks, nil
}

func (r *metadataRepository) GetCrossReference(ctx context.Context, ref models.CrossTableRef) ([]models.ForeignKey, error) {
	const q = `
SELECT 
    pc.owner as pk_owner,
    pc.table_name as pk_table,
    pcc.column_name as pk_column,
    fc.owner as fk_owner,
    fc.table_name as fk_table,
    fcc.column_name as fk_column,
    fcc.position,
    fc.constraint_name as fk_name,
    pc.constraint_name as pk_name,
    fc.delete_rule
FROM   all_constraints pc
JOIN   all_cons_columns pcc ON pc.constraint_name = pcc.constraint_name 
                            AND pc.owner = pcc.owner
JOIN   all_constraints fc ON pc.constraint_name = fc.r_constraint_name 
                          AND pc.owner = fc.r_owner
JOIN   all_cons_columns fcc ON fc.constraint_name = fcc.constraint_name 
                            AND fc.owner = fcc.owner 
                            AND pcc.position = fcc.position
WHERE  pc.constraint_type = 'P'
AND    fc.constraint_type = 'R'
AND    pc.table_name = :1
AND    fc.table_name = :2`

	args := []interface{}{strings.ToUpper(ref.PKRef.Table), strings.ToUpper(ref.FKRef.Table)}
	argCount := 3

	db, err := r.conn(ctx)
	if err != nil {
		return nil, err
	}

	query := q
	if s := strPtr(ref.PKRef.DBSchema); s != "" {
		query += fmt.Sprintf(" AND pc.owner = :%d", argCount)
		args = append(args, strings.ToUpper(s))
		argCount++
	}
	if s := strPtr(ref.FKRef.DBSchema); s != "" {
		query += fmt.Sprintf(" AND fc.owner = :%d", argCount)
		args = append(args, strings.ToUpper(s))
		argCount++
	}
	query += " ORDER BY fc.constraint_name, fcc.position"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, r.wrapDBErr(err, query)
	}
	defer rows.Close()

	var fks []models.ForeignKey
	catalog := "ORACLE"

	for rows.Next() {
		var pkOwner, pkTable, pkColumn, fkOwner, fkTable, fkColumn, fkName, pkName, deleteRule string
		var position int32
		if err := rows.Scan(&pkOwner, &pkTable, &pkColumn, &fkOwner, &fkTable, &fkColumn, &position, &fkName, &pkName, &deleteRule); err != nil {
			return nil, err
		}

		fks = append(fks, models.ForeignKey{
			PKCatalogName: catalog,
			PKSchemaName:  pkOwner,
			PKTableName:   pkTable,
			PKColumnName:  pkColumn,
			FKCatalogName: catalog,
			FKSchemaName:  fkOwner,
			FKTableName:   fkTable,
			FKColumnName:  fkColumn,
			KeySequence:   position,
			PKKeyName:     pkName,
			FKKeyName:     fkName,
			UpdateRule:    models.FKRuleNoAction,
			DeleteRule:    toFKRule(deleteRule),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return fks, nil
}

func (r *metadataRepository) GetTypeInfo(ctx context.Context, dataType *int32) ([]models.XdbcTypeInfo, error) {
	tc := converter.New(r.log)

	// Define common Oracle types
	oracleTypes := []string{
		"NUMBER", "FLOAT", "BINARY_FLOAT", "BINARY_DOUBLE",
		"CHAR", "VARCHAR2", "NCHAR", "NVARCHAR2", "CLOB", "NCLOB",
		"BLOB", "RAW", "LONG RAW",
		"DATE", "TIMESTAMP", "TIMESTAMP WITH TIME ZONE", "TIMESTAMP WITH LOCAL TIME ZONE",
		"INTERVAL YEAR TO MONTH", "INTERVAL DAY TO SECOND",
		"ROWID", "UROWID",
	}

	// If dataType is nil, return all types
	if dataType == nil {
		types := make([]models.XdbcTypeInfo, 0)
		for _, oracleType := range oracleTypes {
			sqlType := tc.GetSQLType(oracleType)
			arrowType, err := tc.DuckDBToArrowType(oracleType)
			if err != nil {
				// Map Oracle type to generic Arrow type if not found
				arrowType = arrow.BinaryTypes.String
			}

			types = append(types, models.XdbcTypeInfo{
				TypeName:          oracleType,
				DataType:          sqlType,
				ColumnSize:        sql.NullInt32{Int32: getOracleColumnSize(oracleType), Valid: true},
				LiteralPrefix:     sql.NullString{String: getLiteralPrefix(arrowType), Valid: true},
				LiteralSuffix:     sql.NullString{String: getLiteralSuffix(arrowType), Valid: true},
				CreateParams:      sql.NullString{String: getOracleCreateParams(oracleType), Valid: true},
				Nullable:          1, // SQL_NULLABLE
				CaseSensitive:     getCaseSensitive(arrowType),
				Searchable:        3, // SQL_SEARCHABLE
				UnsignedAttribute: sql.NullBool{Bool: false, Valid: true},
				FixedPrecScale:    getFixedPrecScale(arrowType),
				AutoIncrement:     sql.NullBool{Bool: false, Valid: true},
				LocalTypeName:     sql.NullString{String: oracleType, Valid: true},
				MinimumScale:      sql.NullInt32{Int32: getMinimumScale(arrowType), Valid: true},
				MaximumScale:      sql.NullInt32{Int32: getMaximumScale(arrowType), Valid: true},
				SQLDataType:       sqlType,
				DatetimeSubcode:   sql.NullInt32{Int32: getSQLDateTimeSub(arrowType), Valid: true},
				NumPrecRadix:      sql.NullInt32{Int32: getNumPrecRadix(arrowType), Valid: true},
				IntervalPrecision: sql.NullInt32{Valid: false},
			})
		}
		return types, nil
	}

	// Filter by specific data type
	for _, oracleType := range oracleTypes {
		sqlType := tc.GetSQLType(oracleType)
		if sqlType == *dataType {
			arrowType, err := tc.DuckDBToArrowType(oracleType)
			if err != nil {
				arrowType = arrow.BinaryTypes.String
			}

			return []models.XdbcTypeInfo{{
				TypeName:          oracleType,
				DataType:          sqlType,
				ColumnSize:        sql.NullInt32{Int32: getOracleColumnSize(oracleType), Valid: true},
				LiteralPrefix:     sql.NullString{String: getLiteralPrefix(arrowType), Valid: true},
				LiteralSuffix:     sql.NullString{String: getLiteralSuffix(arrowType), Valid: true},
				CreateParams:      sql.NullString{String: getOracleCreateParams(oracleType), Valid: true},
				Nullable:          1,
				CaseSensitive:     getCaseSensitive(arrowType),
				Searchable:        3,
				UnsignedAttribute: sql.NullBool{Bool: false, Valid: true},
				FixedPrecScale:    getFixedPrecScale(arrowType),
				AutoIncrement:     sql.NullBool{Bool: false, Valid: true},
				LocalTypeName:     sql.NullString{String: oracleType, Valid: true},
				MinimumScale:      sql.NullInt32{Int32: getMinimumScale(arrowType), Valid: true},
				MaximumScale:      sql.NullInt32{Int32: getMaximumScale(arrowType), Valid: true},
				SQLDataType:       sqlType,
				DatetimeSubcode:   sql.NullInt32{Int32: getSQLDateTimeSub(arrowType), Valid: true},
				NumPrecRadix:      sql.NullInt32{Int32: getNumPrecRadix(arrowType), Valid: true},
				IntervalPrecision: sql.NullInt32{Valid: false},
			}}, nil
		}
	}

	return []models.XdbcTypeInfo{}, nil
}

func (r *metadataRepository) GetSQLInfo(ctx context.Context, ids []uint32) ([]models.SQLInfo, error) {
	if r.sqlInfo == nil {
		return nil, errors.New(errors.CodeInternal, "SQL info provider not configured")
	}
	return r.sqlInfo.GetSQLInfo(ids)
}

//───────────────────────────────────
// internal helpers
//───────────────────────────────────

func (r *metadataRepository) conn(ctx context.Context) (*sql.Conn, error) {
	db, err := r.pool.Get(ctx)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeConnectionFailed, "get conn")
	}
	return db.Conn(ctx)
}

func (r *metadataRepository) wrapDBErr(err error, sql string) error {
	return errors.Wrap(err, errors.CodeQueryFailed, fmt.Sprintf("query: %s", truncateSQL(sql, 100)))
}

// Helper functions
func scanSchemas(rows *sql.Rows, catalog string) ([]models.Schema, error) {
	var schemas []models.Schema
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		schemas = append(schemas, models.Schema{
			CatalogName: catalog,
			Name:        name,
		})
	}
	return schemas, rows.Err()
}

func scanTables(rows *sql.Rows) ([]models.Table, error) {
	var tables []models.Table
	for rows.Next() {
		var catalog, schema, name, tableType string
		if err := rows.Scan(&catalog, &schema, &name, &tableType); err != nil {
			return nil, err
		}
		tables = append(tables, models.Table{
			CatalogName: catalog,
			SchemaName:  schema,
			Name:        name,
			Type:        tableType,
		})
	}
	return tables, rows.Err()
}

func scanColumns(rows *sql.Rows) ([]models.Column, error) {
	var columns []models.Column
	for rows.Next() {
		var catalog, schema, table, name sql.NullString
		var position sql.NullInt32
		var defaultVal, nullable, dataType sql.NullString
		var charMaxLen, numPrec, numScale, dtPrec sql.NullInt32

		if err := rows.Scan(&catalog, &schema, &table, &name, &position, &defaultVal, &nullable, &dataType, &charMaxLen, &numPrec, &numScale, &dtPrec); err != nil {
			return nil, err
		}

		columns = append(columns, models.Column{
			CatalogName:       catalog.String,
			SchemaName:        schema.String,
			TableName:         table.String,
			Name:              name.String,
			OrdinalPosition:   int(position.Int32),
			DefaultValue:      defaultVal,
			IsNullable:        nullable.String == "YES",
			DataType:          dataType.String,
			CharMaxLength:     sql.NullInt64{Int64: int64(charMaxLen.Int32), Valid: charMaxLen.Valid},
			NumericPrecision:  sql.NullInt64{Int64: int64(numPrec.Int32), Valid: numPrec.Valid},
			NumericScale:      sql.NullInt64{Int64: int64(numScale.Int32), Valid: numScale.Valid},
			DateTimePrecision: sql.NullInt64{Int64: int64(dtPrec.Int32), Valid: dtPrec.Valid},
		})
	}
	return columns, rows.Err()
}

// Utility functions
func like(s string) string {
	if s == "" {
		return "%"
	}
	return s
}

func likeDeref(ptr *string) string { return like(strPtr(ptr)) }
func isWild(s string) bool         { return s == "" || s == "%" }
func strPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func truncateSQL(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func toFKRule(rule string) models.FKRule {
	switch strings.ToUpper(rule) {
	case "CASCADE":
		return models.FKRuleCascade
	case "SET NULL":
		return models.FKRuleSetNull
	default:
		return models.FKRuleNoAction
	}
}

// Oracle-specific helper functions
func getOracleColumnSize(oracleType string) int32 {
	switch oracleType {
	case "NUMBER":
		return 38
	case "FLOAT":
		return 126
	case "BINARY_FLOAT":
		return 7
	case "BINARY_DOUBLE":
		return 15
	case "CHAR", "NCHAR":
		return 2000
	case "VARCHAR2", "NVARCHAR2":
		return 4000
	case "CLOB", "NCLOB", "BLOB":
		return 2147483647 // Max CLOB size
	case "RAW":
		return 2000
	case "LONG RAW":
		return 2147483647
	case "DATE":
		return 7
	case "TIMESTAMP", "TIMESTAMP WITH TIME ZONE", "TIMESTAMP WITH LOCAL TIME ZONE":
		return 11
	default:
		return 0
	}
}

func getOracleCreateParams(oracleType string) string {
	switch oracleType {
	case "NUMBER":
		return "precision,scale"
	case "CHAR", "NCHAR", "VARCHAR2", "NVARCHAR2", "RAW":
		return "length"
	case "TIMESTAMP", "TIMESTAMP WITH TIME ZONE", "TIMESTAMP WITH LOCAL TIME ZONE":
		return "precision"
	case "INTERVAL YEAR TO MONTH", "INTERVAL DAY TO SECOND":
		return "precision"
	default:
		return ""
	}
}

// Arrow type helper functions
func getLiteralPrefix(arrowType arrow.DataType) string {
	switch arrowType.ID() {
	case arrow.STRING:
		return "'"
	default:
		return ""
	}
}

func getLiteralSuffix(arrowType arrow.DataType) string {
	switch arrowType.ID() {
	case arrow.STRING:
		return "'"
	default:
		return ""
	}
}

func getCaseSensitive(arrowType arrow.DataType) bool {
	return arrowType.ID() == arrow.STRING
}

func getFixedPrecScale(arrowType arrow.DataType) bool {
	switch arrowType.ID() {
	case arrow.DECIMAL128, arrow.DECIMAL256:
		return true
	default:
		return false
	}
}

func getMinimumScale(arrowType arrow.DataType) int32 {
	switch arrowType.ID() {
	case arrow.DECIMAL128, arrow.DECIMAL256:
		return 0
	default:
		return 0
	}
}

func getMaximumScale(arrowType arrow.DataType) int32 {
	switch arrowType.ID() {
	case arrow.DECIMAL128, arrow.DECIMAL256:
		return 38
	default:
		return 0
	}
}

func getSQLDateTimeSub(arrowType arrow.DataType) int32 {
	switch arrowType.ID() {
	case arrow.DATE32, arrow.DATE64:
		return 1 // SQL_CODE_DATE
	case arrow.TIME32, arrow.TIME64:
		return 2 // SQL_CODE_TIME
	case arrow.TIMESTAMP:
		return 3 // SQL_CODE_TIMESTAMP
	default:
		return 0
	}
}

func getNumPrecRadix(arrowType arrow.DataType) int32 {
	switch arrowType.ID() {
	case arrow.INT8, arrow.INT16, arrow.INT32, arrow.INT64,
		arrow.UINT8, arrow.UINT16, arrow.UINT32, arrow.UINT64,
		arrow.FLOAT32, arrow.FLOAT64,
		arrow.DECIMAL128, arrow.DECIMAL256:
		return 10
	default:
		return 0
	}
}
