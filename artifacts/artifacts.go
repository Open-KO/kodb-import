package artifacts

import (
	"fmt"
	"kodb-import/config"
	"kodb-import/mssql"
	"os"
	"path/filepath"
)

// the artifacts package contains reference constants and helpers that map to the OpenKO-db project
// This package shouldn't import any other packages in this project to avoid circular dependencies.
// Exception: config package

const (

	// directory constants for using the OpenKO-db project
	TemplatesDir   = "Templates"
	ViewsDir       = "Views"
	StoredProcsDir = "StoredProcedures"
	ManualSetupDir = "ManualSetup"

	// template files used to generate several structural exports

	CreateDatabaseTemplate = "CreateDatabase.sqltemplate"
	CreateUserTemplate     = "CreateUser.sqltemplate"
	CreateLoginTemplate    = "CreateLogin.sqltemplate"
	CreateSchemaTemplate   = "CreateSchema.sqltemplate"

	// script file name formats:  [Step]_Create[Type]_[DbType.String()]_[ArtifactName].sql

	CreateDatabaseFileNameFmt        = "1_CreateDatabase_%s.sql"
	CreateSchemaFileNameFmt          = "2_CreateSchema_%s.sql"
	CreateUserFileNameFmt            = "3_CreateUser_%s.sql"
	CreateLoginFileNameFmt           = "4_CreateLogin_%s.sql"
	CreateTableFileNameFmt           = "5_CreateTable_%s.sql"
	CreateTableDataFileNameFmt       = "6_InsertData_%s.sql"
	CreateViewFileNameFmt            = "7_CreateView_%s.sql"
	CreateStoredProcedureFileNameFmt = "8_CreateStoredProc_%s.sql"
)

// GetCreateDatabaseScript loads the CreateDatabase template, substitutes variables, and returns the sql script as a string
func GetCreateDatabaseScript(driver *mssql.MssqlDbDriver) (script string, err error) {
	sqlFmtBytes, err := os.ReadFile(filepath.Join(config.GetConfig().GenConfig.SchemaDir, TemplatesDir, CreateDatabaseTemplate))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(string(sqlFmtBytes), driver.GenDbConfig.Name), nil
}

// GetCreateLoginScript loads the CreateLogin template, substitutes variables, and returns the sql script as a string
func GetCreateLoginScript(driver *mssql.MssqlDbDriver, loginIndex int) (script string, err error) {
	sqlFmtBytes, err := os.ReadFile(filepath.Join(config.GetConfig().GenConfig.SchemaDir, TemplatesDir, CreateLoginTemplate))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(string(sqlFmtBytes), driver.GenDbConfig.Logins[loginIndex].Name, driver.GenDbConfig.Name, driver.GenDbConfig.Logins[loginIndex].Pass), nil
}

// GetCreateUserScript loads the CreateUser template, substitutes variables, and returns the sql script as a string
func GetCreateUserScript(driver *mssql.MssqlDbDriver, userIndex int) (script string, err error) {
	sqlFmtBytes, err := os.ReadFile(filepath.Join(config.GetConfig().GenConfig.SchemaDir, TemplatesDir, CreateUserTemplate))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(string(sqlFmtBytes), driver.GenDbConfig.Users[userIndex].Name, driver.GenDbConfig.Users[userIndex].Schema, driver.GenDbConfig.Name), nil
}

// GetCreateSchemaScript loads the CreateSchema template, substitutes variables, and returns the sql script as a string
func GetCreateSchemaScript(driver *mssql.MssqlDbDriver, schemaIndex int) (script string, err error) {
	sqlFmtBytes, err := os.ReadFile(filepath.Join(config.GetConfig().GenConfig.SchemaDir, TemplatesDir, CreateSchemaTemplate))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(string(sqlFmtBytes), driver.GenDbConfig.Schemas[schemaIndex], driver.GenDbConfig.Name), nil
}
