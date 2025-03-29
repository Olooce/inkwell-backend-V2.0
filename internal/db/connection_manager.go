package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"inkwell-backend-V2.0/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Conn is the shared database connection.
var Conn *gorm.DB

// PingInterval is the interval for checking database connection health.
const PingInterval = 30 * time.Second

// InitDBFromConfig initializes a connection pool using settings from the config file.
func InitDBFromConfig(cfg *config.APIConfig) {
	fmt.Println("\nCONNECTION MANAGER - INKWELL-API-CM")
	fmt.Println("-------------[Creating Connection Pool...]-----------")
	fmt.Printf(" Database Server       : %s\n", cfg.DB.Server)
	fmt.Printf(" Database Driver       : %s\n", cfg.DB.Driver)
	fmt.Printf(" Database Name         : %s\n", cfg.DB.Names.INKWELL)
	fmt.Printf(" Database Username     : %s\n", cfg.DB.Username)
	fmt.Printf(" Database Port         : %d\n", cfg.DB.Port)

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
		cfg.DB.Host,
		cfg.DB.Username,
		cfg.DB.Password.Value,
		cfg.DB.Names.INKWELL,
		cfg.DB.Port,
		cfg.DB.SSLMode,
		cfg.Context.TimeZone,
	)

	var err error
	Conn, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	sqlDB, err := Conn.DB()
	if err != nil {
		log.Fatalf("Failed to get database connection: %v", err)
	}

	sqlDB.SetMaxOpenConns(cfg.DB.Pool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DB.Pool.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.DB.Pool.ConnMaxLifetime) * time.Second)

	go monitorConnectionPool(sqlDB)

	printConnectionPoolStats(sqlDB, cfg)

	fmt.Println("\nConnection Pool Initialized Successfully!")
}

// monitorConnectionPool checks DB health periodically.
func monitorConnectionPool(db *sql.DB) {
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()

	for range ticker.C {
		if err := db.Ping(); err != nil {
			log.Printf("Database connection unhealthy: %v\n", err)
			ReconnectDB()
		}
	}
}

// ReconnectDB reloads the connection dynamically.
func ReconnectDB() {
	log.Println("Recreating database connection...")

	cfg := config.GetConfig()
	if cfg == nil {
		log.Println("Failed to reload config: using existing settings")
		return
	}

	InitDBFromConfig(cfg)
}

// printConnectionPoolStats displays pool stats.
func printConnectionPoolStats(db *sql.DB, cfg *config.APIConfig) {
	stats := db.Stats()

	fmt.Println("\n-------------[Connection Pool Details]-------------")
	fmt.Printf(" Max Open Connections          : %d\n", cfg.DB.Pool.MaxOpenConns)
	fmt.Printf(" Max Idle Connections          : %d\n", cfg.DB.Pool.MaxIdleConns)
	fmt.Printf(" Connection Max Lifetime       : %d seconds\n", cfg.DB.Pool.ConnMaxLifetime)
	fmt.Println("\n-------------[Current Pool Status]-------------")
	fmt.Printf(" Open Connections              : %d\n", stats.OpenConnections)
	fmt.Printf(" In Use Connections            : %d\n", stats.InUse)
	fmt.Printf(" Idle Connections              : %d\n", stats.Idle)
	fmt.Printf(" Wait Count                    : %d\n", stats.WaitCount)
	fmt.Printf(" Wait Duration (ms)            : %d\n", stats.WaitDuration.Milliseconds())
	fmt.Printf(" Max Idle Closed               : %d\n", stats.MaxIdleClosed)
	fmt.Printf(" Max Lifetime Closed           : %d\n", stats.MaxLifetimeClosed)
	fmt.Println("\n------------------------------------------------")
}

// GetDB returns the current database connection.
func GetDB() *gorm.DB {
	return Conn
}
