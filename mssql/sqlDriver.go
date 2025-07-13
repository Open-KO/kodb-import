package mssql

import (
	"fmt"
	"kodb-import/config"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/Open-KO/kodb-godef/enums/dbType"
	_ "github.com/microsoft/go-mssqldb"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// mssql sql driver impl, see: https://github.com/denisenkom/go-mssqldb

const (
	// driverName is required by the sql.Open() function to identify the go-mssqldb driver
	driverName = "sqlserver"

	// 1: Username
	// 2: Password
	// 3: Host
	// 4: Port
	// 5: Instance Name
	// 6: Database Name
	// connStringFmt is the connection string format used when a dbuser/dbpass are specified
	connStringFmt = "sqlserver://%[1]s:%[2]s@%[3]s:%[4]d/%[5]s?database=%[6]s"

	// 1: Host
	// 2: Port
	// 2: Instance
	// 3: Database
	// winAuthConnStrFmt is the connection string format used when no dbuser is specified; Windows Authentication
	winAuthConnStrFmt = "sqlserver://@%[1]s:%[2]d/%[3]s?database=%[4]s"

	// DefaultSysDbName is the name of the main system database in MSSQL Server; used for database creation queries
	DefaultSysDbName = "master"

	// SqlExtPattern is used to search the filesystem for SQL files
	SqlExtPattern = "*.sql"

	// BatchTerminator is an MS SQL specific keyword that only works when executed from SQL Server Management Studio
	BatchTerminator = "\nGO"
)

// MssqlDbDriver contains information needed to perform our application's SQL connections
type MssqlDbDriver struct {
	dbConfig    config.DatabaseConfig
	GenDbConfig config.GenDbConfig
	DbType      dbType.DbType
	connString  string
	conn        *gorm.DB
	masterConn  *gorm.DB
	tx          *gorm.DB
}

// NewMssqlDbDriver returns an instance of MssqlDbDriver populated with GenDbConfig for a particular database connection
func NewMssqlDbDriver(dbConfig config.GenDbConfig, databaseType dbType.DbType) *MssqlDbDriver {
	return &MssqlDbDriver{
		dbConfig:    config.GetConfig().DatabaseConfig,
		GenDbConfig: dbConfig,
		DbType:      databaseType,
	}
}

// GetConnectionString returns a formatted connection string using the configurations on MssqlDbDriver
func (this *MssqlDbDriver) GetConnectionString(dbName string) string {
	if this.dbConfig.User == "" {
		// Attempt Windows Auth
		this.connString = fmt.Sprintf(winAuthConnStrFmt, this.dbConfig.Host, this.dbConfig.Port, this.dbConfig.Instance, dbName)
	} else {
		// Used Mixed Auth
		this.connString = fmt.Sprintf(connStringFmt, this.dbConfig.User, url.QueryEscape(this.dbConfig.Password), this.dbConfig.Host, this.dbConfig.Port, this.dbConfig.Instance, dbName)
	}

	return this.connString
}

// GetConnection returns a *gorm.DB instance from a connection string generated using the configuration on MssqlDbDriver
func (this *MssqlDbDriver) GetConnection() (*gorm.DB, error) {
	// if there's an existing connection, re-use it
	if this.conn != nil {
		// if there's an open session, use it
		if this.tx != nil {
			return this.tx, nil
		}
		return this.conn, nil
	}

	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,   // Slow SQL threshold
			LogLevel:                  logger.Silent, // Log level
			IgnoreRecordNotFoundError: true,          // Ignore ErrRecordNotFound error for logger
			ParameterizedQueries:      true,          // Don't include params in the SQL log
			Colorful:                  false,         // Disable color
		},
	)

	gormConfig := &gorm.Config{
		Logger: gormLogger,
	}

	// open a connection against the master db
	var err error
	this.conn, err = gorm.Open(sqlserver.Open(this.GetConnectionString(this.GenDbConfig.Name)), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection to mssql: %v", err)
	}

	return this.conn, nil
}

func (this *MssqlDbDriver) GetMasterConnection() (*gorm.DB, error) {
	// if there's an existing connection, re-use it
	if this.masterConn != nil {
		return this.masterConn, nil
	}

	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,   // Slow SQL threshold
			LogLevel:                  logger.Silent, // Log level
			IgnoreRecordNotFoundError: true,          // Ignore ErrRecordNotFound error for logger
			ParameterizedQueries:      true,          // Don't include params in the SQL log
			Colorful:                  false,         // Disable color
		},
	)

	gormConfig := &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: true,
	}

	// open a connection against the master db
	var err error
	this.masterConn, err = gorm.Open(sqlserver.Open(this.GetConnectionString(DefaultSysDbName)), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection to mssql: %v", err)
	}

	return this.masterConn, nil
}

func (this *MssqlDbDriver) GetDb() *gorm.DB {
	return this.conn
}

// GetTx returns the top-level transaction fence for this driver
func (this *MssqlDbDriver) GetTx() (tx *gorm.DB, err error) {
	if this.conn == nil {
		this.conn, err = this.GetConnection()
		if err != nil {
			return nil, err
		}
	}
	if this.tx == nil {
		this.tx = this.conn.Begin()
	}

	return this.tx, nil
}

// CommitTx attempts to commit the top level transaction fence for this driver
func (this *MssqlDbDriver) CommitTx() error {
	if this.tx != nil {
		return this.tx.Commit().Error
	}
	return fmt.Errorf("no transaction to commit")
}

// RollbackTx attempts to rollback the top level transaction fence for this driver
func (this *MssqlDbDriver) RollbackTx() error {
	if this.tx != nil {
		return this.tx.Rollback().Error
	}
	return fmt.Errorf("no transaction to rollback")
}

// CloseConnection nulls the connection pointer pointer; gorm doesn't require manual connection closes
func (this *MssqlDbDriver) CloseConnection() {
	this.conn = nil
}
