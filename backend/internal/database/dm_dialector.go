package database

import (
	"database/sql"
	"fmt"
	"strings"

	_ "gitee.com/chunanyong/dm" // 达梦驱动，init() 自动注册 sql.Open("dm", ...)
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/migrator"
	"gorm.io/gorm/schema"
)
//
// 基于 gitee.com/chunanyong/dm 纯 Go 驱动（支持交叉编译到 ARM64/LoongArch）。
// DSN 格式: dm://user:passwo...ost:port
// 注意: 生产环境建议达梦开启 PG 兼容参数（COMPATIBLE_MODE=4），
// 以获得更好的 SQL 兼容性；表结构可用 migrations/001_init.sql 手工初始化。

type DMDialector struct {
	DSN string
}

// OpenDM 创建达梦方言
func OpenDM(dsn string) gorm.Dialector {
	return &DMDialector{DSN: dsn}
}

func (d *DMDialector) Name() string { return "dm" }

func (d *DMDialector) Initialize(db *gorm.DB) error {
	sqlDB, err := sql.Open("dm", d.DSN)
	if err != nil {
		return fmt.Errorf("达梦数据库连接失败: %w", err)
	}
	db.ConnPool = sqlDB

	// 注册 GORM 默认回调（与 sqlite 方言同构，使用 ? 占位符）
	callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{
		CreateClauses: []string{"INSERT", "VALUES", "ON CONFLICT"},
		UpdateClauses: []string{"UPDATE", "SET", "FROM", "WHERE"},
		DeleteClauses: []string{"DELETE", "FROM", "WHERE"},
	})
	return nil
}

func (d *DMDialector) Migrator(db *gorm.DB) gorm.Migrator {
	return migrator.Migrator{Config: migrator.Config{DB: db, Dialector: d}}
}

// DataTypeOf GORM 字段类型 -> 达梦类型映射
func (d *DMDialector) DataTypeOf(field *schema.Field) string {
	switch field.DataType {
	case schema.Bool:
		return "INT" // 达梦无原生 bool，用 0/1
	case schema.Int, schema.Uint:
		if field.AutoIncrement {
			return "BIGINT IDENTITY(1,1)"
		}
		switch {
		case field.Size <= 16:
			return "SMALLINT"
		case field.Size <= 32:
			return "INT"
		default:
			return "BIGINT"
		}
	case schema.Float:
		if field.Precision > 0 {
			return fmt.Sprintf("DECIMAL(%d,%d)", field.Precision, field.Scale)
		}
		return "DOUBLE"
	case schema.String:
		size := field.Size
		if size == 0 {
			size = 255
		}
		if size > 8000 {
			return "CLOB"
		}
		return fmt.Sprintf("VARCHAR(%d)", size)
	case schema.Time:
		return "TIMESTAMP"
	case schema.Bytes:
		return "BLOB"
	default:
		return "VARCHAR(255)"
	}
}

func (d *DMDialector) DefaultValueOf(field *schema.Field) clause.Expression {
	if field.DefaultValueInterface != nil {
		return clause.Expr{SQL: "?", Vars: []interface{}{field.DefaultValueInterface}}
	}
	return clause.Expr{SQL: "DEFAULT"}
}

func (d *DMDialector) BindVarTo(writer clause.Writer, stmt *gorm.Statement, v interface{}) {
	writer.WriteByte('?')
}

func (d *DMDialector) QuoteTo(writer clause.Writer, str string) {
	writer.WriteByte('"')
	if strings.Contains(str, ".") {
		for idx, s := range strings.Split(str, ".") {
			if idx > 0 {
				writer.WriteString(`."`)
			}
			writer.WriteString(s)
			writer.WriteByte('"')
		}
	} else {
		writer.WriteString(str)
		writer.WriteByte('"')
	}
}

func (d *DMDialector) Explain(sql string, vars ...interface{}) string {
	return fmt.Sprintf(strings.ReplaceAll(sql, "?", "'%v'"), vars...)
}
