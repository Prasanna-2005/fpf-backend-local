/*
Package db provides database connection and management functionality using GORM with MySQL.
*/
package db

import (
	"context"
	"database/sql"
	"fmt"
	"form_pf_backend/config"
	"log"
	"os"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	sqlDb *gorm.DB  // DB holds the global database connection instance
	once  sync.Once // once ensures the database is initialized only once
)

/*
Connect establishes a connection to the MySQL database using environment variables.

This function reads database configuration from environment variables and creates
a GORM database connection with MySQL driver.

Environment Variables Required:
- DB_USER: Database username
- DB_PASSWORD: Database password
- DB_HOST: Database host address
- DB_PORT: Database port number
- DB_NAME: Database name

The function will terminate the application if connection fails.
*/
func connect() {
	var err error
	// checkConnLiveness checks the socket before reuse. MySQL on the VM closes an idle
	// connection after wait_timeout, and a restarted VM drops the TCP session.
	// A dead socket is discarded here instead of failing the next query.
	connectionString := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&checkConnLiveness=true",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"))

	sqlDb, err = gorm.Open(mysql.Open(connectionString), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if rawDB, err := sqlDb.DB(); err == nil {
		pool := config.LoadDBPool()
		rawDB.SetMaxOpenConns(pool.MaxOpenConns)
		rawDB.SetMaxIdleConns(pool.MaxOpenConns)
		rawDB.SetConnMaxLifetime(pool.ConnMaxLifetime)
		rawDB.SetConnMaxIdleTime(pool.ConnMaxIdleTime)
		go keepPoolWarm(rawDB, pool.PingInterval, pool.PingTimeout)
	}

	// Migrate the schema
	// DB.AutoMigrate(&models.User{})
}

func keepPoolWarm(rawDB *sql.DB, every, timeout time.Duration) {
	if every <= 0 {
		every = 30 * time.Second
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err := rawDB.PingContext(ctx)
		cancel()
		if err != nil {
			log.Printf("db pool ping: %v", err)
		}
	}
}

func Init() *gorm.DB {
	/*
		Init returns the global database connection instance.

		:returns: Pointer to the GORM database instance
		:rtype: *gorm.DB
	*/
	connect()
	return sqlDb
}

func GetDB() *gorm.DB {
	/*
		GetDB returns the global database connection instance, initializing it if needed.

		This function uses sync.Once to ensure the database connection is established
		only once, making it safe for concurrent use.

		:returns: Pointer to the GORM database instance
		:rtype: *gorm.DB
	*/
	once.Do(func() {
		connect()
	})
	return sqlDb
}

// returns true if the handle has not recorded any error
func dbAlive(db *gorm.DB) bool { return db.Error == nil }
