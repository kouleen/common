package mysql

import (
	"fmt"
	"log"
	"os"
	"sync/atomic"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	mysqlWriteDB   *gorm.DB
	mysqlReadDB1   *gorm.DB
	mysqlReadDB2   *gorm.DB
	readRoundRobin uint64
)

func init() {
	if os.Getenv("MYSQL_USERNAME") == "" {
		log.Fatal("MYSQL_USERNAME env variable not set")
	}
	if os.Getenv("MYSQL_PASSWORD") == "" {
		log.Fatal("MYSQL_PASSWORD env variable not set")
	}
	if os.Getenv("MYSQL_HOST") == "" {
		if err := os.Setenv("MYSQL_HOST", "mysql"); err != nil {
			log.Fatal(err)
		}
	}
	if os.Getenv("MYSQL_PORT") == "" {
		if err := os.Setenv("MYSQL_PORT", "3306"); err != nil {
			log.Fatal(err)
		}
	}
	if os.Getenv("MYSQL_DATABASE") == "" {
		log.Fatal("MYSQL_DATABASE env variable not set")
	}
	if err := initWriteMysql(os.Getenv("MYSQL_USERNAME"), os.Getenv("MYSQL_PASSWORD"), os.Getenv("MYSQL_HOST"), os.Getenv("MYSQL_PORT"), os.Getenv("MYSQL_DATABASE")); err != nil {
		log.Fatal(err)
	}

	if os.Getenv("MYSQL_SLAVE1_USERNAME") != "" {
		if os.Getenv("MYSQL_SLAVE1_PASSWORD") == "" {
			log.Fatal("MYSQL_PASSWORD env variable not set")
		}
		if os.Getenv("MYSQL_SLAVE1_HOST") == "" {
			log.Fatal("MYSQL_HOST env variable not set")
		}
		if os.Getenv("MYSQL_SLAVE1_PORT") == "" {
			log.Fatal("MYSQL_PORT env variable not set")
		}
		if err := initRead1Mysql(os.Getenv("MYSQL_SLAVE1_USERNAME"), os.Getenv("MYSQL_SLAVE1_PASSWORD"), os.Getenv("MYSQL_SLAVE1_HOST"), os.Getenv("MYSQL_SLAVE1_PORT"), os.Getenv("MYSQL_DATABASE")); err != nil {
			log.Fatal(err)
		}
	}
	if os.Getenv("MYSQL_SLAVE2_USERNAME") != "" {
		if os.Getenv("MYSQL_SLAVE2_PASSWORD") == "" {
			log.Fatal("MYSQL_PASSWORD env variable not set")
		}
		if os.Getenv("MYSQL_SLAVE2_HOST") == "" {
			log.Fatal("MYSQL_HOST env variable not set")
		}
		if os.Getenv("MYSQL_SLAVE2_PORT") == "" {
			log.Fatal("MYSQL_PORT env variable not set")
		}
		if err := initRead2Mysql(os.Getenv("MYSQL_SLAVE2_USERNAME"), os.Getenv("MYSQL_SLAVE2_PASSWORD"), os.Getenv("MYSQL_SLAVE2_HOST"), os.Getenv("MYSQL_SLAVE2_PORT"), os.Getenv("MYSQL_DATABASE")); err != nil {
			log.Fatal(err)
		}
	}
}

func initWriteMysql(username, password, host, port, database string) error {
	dns := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", username, password, host, port, database)
	db, err := gorm.Open(mysql.Open(dns), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	sqlDB.SetConnMaxIdleTime(30 * time.Second)
	mysqlWriteDB = db
	return nil
}

func initRead1Mysql(username, password, host, port, database string) error {
	dns := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", username, password, host, port, database)
	db, err := gorm.Open(mysql.Open(dns), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	sqlDB.SetConnMaxIdleTime(30 * time.Second)
	mysqlReadDB1 = db
	return nil
}

func initRead2Mysql(username, password, host, port, database string) error {
	dns := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", username, password, host, port, database)
	db, err := gorm.Open(mysql.Open(dns), &gorm.Config{})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(30 * time.Second)
	sqlDB.SetConnMaxIdleTime(30 * time.Second)
	mysqlReadDB2 = db
	return nil
}

func GetWriteMysqlDDB() *gorm.DB {
	return mysqlWriteDB
}

func GetReadMysqlDDB() *gorm.DB {
	if mysqlReadDB1 == nil || mysqlReadDB2 == nil {
		return mysqlWriteDB
	}
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
