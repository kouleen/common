package sqlite

import (
	"gorm.io/gorm"
)

func GetSqliteClient() *gorm.DB {
	return sqliteDB
}
