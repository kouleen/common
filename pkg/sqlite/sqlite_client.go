package sqlite

import (
	"log"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var sqliteDB *gorm.DB

func init() {
	if os.Getenv("SQLITE_DATABASE") == "" {
		log.Fatal("SQLITE_DATABASE env variable not set")
	}
	initSqliteDB(os.Getenv("SQLITE_DATABASE"))
}

// 会自动在项目根目录创建 data.strategy 文件
func initSqliteDB(database string) {
	// 连接 SQLite（文件不存在会自动创建）
	db, err := gorm.Open(sqlite.Open(database+"?_loc=Local&parseTime=true&_journal_mode=WAL&_cache_size=-20000"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// 获取底层 DB 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}

	// SQLite 连接池配置（轻量即可）
	sqlDB.SetMaxOpenConns(1)    // 最大打开连接
	sqlDB.SetMaxIdleConns(1)    // 最大空闲连接
	sqlDB.SetConnMaxLifetime(0) // 连接永不过期

	// 赋值给外部指针
	sqliteDB = db
}

func GetSqliteDb() *gorm.DB {
	return sqliteDB
}
