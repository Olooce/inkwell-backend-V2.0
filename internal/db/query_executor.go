package db

import "gorm.io/gorm"

// GetDB returns the current database connection.
func GetDB() *gorm.DB {
	return Conn
}
