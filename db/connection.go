/*
Package db provides database connection and management functionality using GORM with MySQL.
*/
package db

import (
	"fmt"
	"log"
	"os"
	"sync"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var (
	DB   *gorm.DB  // DB holds the global database connection instance
	once sync.Once // once ensures the database is initialized only once
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
	connectionString := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"))

	DB, err = gorm.Open(mysql.Open(connectionString), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// Migrate the schema
	// DB.AutoMigrate(&models.User{})
}

/*
Init returns the global database connection instance.

:returns: Pointer to the GORM database instance
:rtype: *gorm.DB
*/

func Init() *gorm.DB {
	connect()
	return DB
}

/*
GetDB returns the global database connection instance, initializing it if needed.

This function uses sync.Once to ensure the database connection is established
only once, making it safe for concurrent use.

:returns: Pointer to the GORM database instance
:rtype: *gorm.DB
*/
func GetDB() *gorm.DB {
	once.Do(func() {
		connect()
	})
	return DB
}

/*
CloseDB closes the database connection.

This function should be called during application shutdown to properly
close the database connection and free resources.

:returns: Error if closing fails, nil otherwise
:rtype: error
*/
func CloseDB() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return fmt.Errorf("failed to get underlying sql.DB: %w", err)
		}
		return sqlDB.Close()
	}
	return nil
}
