package mysql

import (
	"sync/atomic"

	"gorm.io/gorm"
)

func GetWriteMysqlDDB() *gorm.DB {
	return mysqlWriteDB
}

func GetReadMysqlDDB() *gorm.DB {
	if mysqlReadDB1 != nil && mysqlReadDB2 != nil {
		// 原子自增
		idx := atomic.AddUint64(&readRoundRobin, 1)
		// 取模轮询 0,1,0,1...
		switch idx % 2 {
		case 0:
			return mysqlReadDB1
		case 1:
			return mysqlReadDB2
		default:
			// 兜底返回第一个
			return mysqlReadDB1
		}
	}
	if mysqlReadDB1 != nil {
		return mysqlReadDB1
	}
	if mysqlReadDB2 != nil {
		return mysqlReadDB2
	}
	return mysqlWriteDB
}
