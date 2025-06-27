/*
Package db provides database connection and management functionality using GORM with MySQL.
*/
package db

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DB holds the global database connection instance
var DB *gorm.DB

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
func Connect() {
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
GetDB returns the global database connection instance.

:returns: Pointer to the GORM database instance
:rtype: *gorm.DB
*/
func GetDB() *gorm.DB {
	return DB
}
