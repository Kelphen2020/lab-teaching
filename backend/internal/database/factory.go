package database

import (
	"fmt"
	"log"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"lab-teaching/backend/internal/config"
)

// ============ V1.0-14 国产数据库兼容 ============
//
// 支持三种数据库类型（配置 database.type）:
//   - postgres : 标准 PostgreSQL（默认，开发/通用环境）
//   - kingbase : 人大金仓 KingbaseES（PG 兼容协议，复用 pgx 驱动）
//   - dm       : 达梦 DM8（纯 Go 驱动，支持国产 CPU 交叉编译）
//
// 国产 CPU 交叉编译说明:
//   - postgres/kingbase 走 pgx 纯 Go 实现，CGO_ENABLED=0 即可交叉编译
//   - dm 驱动 gitee.com/chunanyong/dm 为纯 Go，同样支持交叉编译

// DBType 数据库类型
type DBType string

const (
	DBPostgres DBType = "postgres"
	DBKingbase DBType = "kingbase"
	DBDM       DBType = "dm"
)

// Open 根据配置创建数据库连接，返回 *gorm.DB
func Open(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	dbType := DBType(strings.ToLower(strings.TrimSpace(cfg.Type)))
	if dbType == "" {
		dbType = DBPostgres
	}

	var dialector gorm.Dialector
	switch dbType {
	case DBPostgres, DBKingbase:
		// 人大金仓 KingbaseES V8R6+ 兼容 PostgreSQL 协议，直接使用 pgx 驱动
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)
		dialector = postgres.Open(dsn)
		if dbType == DBKingbase {
			log.Println("[db] 使用人大金仓 KingbaseES (PG 兼容模式)")
		}
	case DBDM:
		// 达梦 DSN: dm://user:passwo…st:port
		dsn := fmt.Sprintf("dm://%s:%s@%s:%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port)
		dialector = OpenDM(dsn)
		log.Println("[db] 使用达梦 DM8")
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s (可选 postgres/kingbase/dm)", cfg.Type)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("数据库连接失败(type=%s): %w", dbType, err)
	}
	return db, nil
}
