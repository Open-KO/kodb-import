package importDb

import (
	"context"
	"fmt"
	"kodb-import/artifacts"
	"kodb-import/config"
	"kodb-import/mssql"
	"kodb-import/utils"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (

	// ImportBatSize is used to set the number of insert records sent in each batch.  Valid values 2-999.
	ImportBatSize = 16

	// this was benchmarked, changing it may cause performance issues:
	//table data successfully imported in 1m37.0268984s; batch size 999
	//table data successfully imported in 1m7.0408631s; batch size 500
	//table data successfully imported in 52.0091316s; batch size 200
	//table data successfully imported in 46.7709405s; batch size 100
	//table data successfully imported in 42.6671243s; batch size 50
	//table data successfully imported in 45.1741919s; batch size 32
	//table data successfully imported in 45.0607315s; batch size 20
	//table data successfully imported in 8.2753s; batch size 16
	//table data successfully imported in 9.0527814s; batch size 10
	//table data successfully imported in 9.6099392s; batch size 8
	//table data successfully imported in 13.9534485s; batch size 4
	//table data successfully imported in 19.8701158s; batch size 2
	// curious how it may run on other machines, particularly ones with different numbers of cores.
	// benchmark data above run on: Intel(R) Core(TM) i9-9900K CPU @ 3.60GHz
)

// Script contains the file Name and Sql contents of a *.sql file
type Script struct {
	Name string
	Sql  string
}

// ScriptArgs are arguments used in the runScripts function
type ScriptArgs struct {
	// IsUseDefaultSystemDb will use the mssql.DefaultSysDbName (master) when true.  Default false
	IsUseDefaultSystemDb bool

	// IsDataDump set to true for loading one of our insert dumps; our dumps do not use "GO" batch separators and must be manually split
	// this is done to keep our insert files diff-friendly and allow us to adjust the ImportBatSize for performance tuning
	IsDataDump bool
}

// defaultScriptArgs returns a ScriptArgs object with default values
func defaultScriptArgs() ScriptArgs {
	return ScriptArgs{
		IsUseDefaultSystemDb: false,
		IsDataDump:           false,
	}
}

// ImportDb attempts to load all *.sql batch files from the OpenKO-db project into an MSSQL instance
// Database creation scripts execute against mssql.DefaultSysDbName, the rest should be
// executed using the created database named in schemaConfig.GameDb.Name
func ImportDb(ctx context.Context, driver *mssql.MssqlDbDriver) (err error) {
	fmt.Println("-- Import --")

	err = importDbs(ctx, driver)
	if err != nil {
		return err
	}

	// open tx to game db
	_, err = driver.GetTx()
	if err != nil {
		return err
	}

	err = importSchemas(ctx, driver)
	if err != nil {
		return err
	}

	err = importUsers(ctx, driver)
	if err != nil {
		return err
	}

	err = importLogins(ctx, driver)
	if err != nil {
		return err
	}

	err = importTables(ctx, driver)
	if err != nil {
		return err
	}

	err = importViews(ctx, driver)
	if err != nil {
		return err
	}

	err = importStoredProcs(ctx, driver)
	if err != nil {
		return err
	}

	return nil
}

// runScripts runs a related group of sql files.  Each file is broken down into batches (separated by the "GO" keyword)
// and then executed/commited within a transaction fence.
func runScripts(ctx context.Context, driver *mssql.MssqlDbDriver, scriptArgs ScriptArgs, sqlScripts ...Script) (err error) {
	if len(sqlScripts) == 0 {
		fmt.Println("WARN: No scripts to execute")
		return nil
	}

	// get gorm connection
	var gormConn *gorm.DB
	if scriptArgs.IsUseDefaultSystemDb {
		gormConn, err = driver.GetMasterConnection()
	} else {
		gormConn, err = driver.GetTx()
	}
	if err != nil {
		return err
	}

	for i := range sqlScripts {
		batches := []string{}
		if scriptArgs.IsDataDump {

			lines := strings.Split(sqlScripts[i].Sql, "\n")
			// sliding window batches
			l := 1
			r := l + ImportBatSize

			header := fmt.Sprintf("%s\n", lines[0])
			for l < len(lines) {
				// put r back on tail element if exceeded
				if r >= len(lines) {
					r = len(lines) - 1
				}

				// remove any trailing "," from previous batch
				if len(batches) > 0 {
					batches[len(batches)-1] = strings.TrimSpace(batches[len(batches)-1])
					batches[len(batches)-1] = strings.TrimSuffix(batches[len(batches)-1], ",")
				}

				if l == r {
					// make sure we didn't just land on the blank line at the end of the file
					if strings.TrimSpace(lines[l]) == "" {
						break
					}
				}

				// capture current window as batch
				// insert header
				batch := header + strings.Join(lines[l:r+1], "\n")
				batches = append(batches, batch)
				l = r + 1
				r += ImportBatSize
			}
		} else {
			batches = splitBatches(sqlScripts[i].Sql)
		}

		for j := range batches {
			err = gormConn.Exec(batches[j]).Error
			if err != nil {
				if !isIgnoreErr(err) {
					fmt.Printf("error executing batch [%d/%d] in %s: %v\n", j+1, len(batches), sqlScripts[i].Name, err)
					fmt.Printf("batch sql: %s", batches[j])
					return err
				} else {
					err = nil
				}
			}
		}
	}

	return nil
}

// importDbs uses the CreateDatabase.sqltemplate to create the database configured in schemaConfig.gameDb
func importDbs(ctx context.Context, driver *mssql.MssqlDbDriver) (err error) {
	defer func() {
		if err == nil {
			fmt.Println("databases successfully imported")
		}
	}()
	fmt.Println("-- Importing databases --")
	sArgs := defaultScriptArgs()
	sArgs.IsUseDefaultSystemDb = true

	script := Script{
		Name: fmt.Sprintf(artifacts.CreateDatabaseFileNameFmt, driver.GenDbConfig.Name),
	}

	script.Sql, err = artifacts.GetCreateDatabaseScript(driver)
	if err != nil {
		return err
	}

	return runScripts(ctx, driver, sArgs, script)
}

// importSchemas uses the CreateSchema.sqltemplate to create schemas defined in schemaConfig.gameDb.schemas
func importSchemas(ctx context.Context, driver *mssql.MssqlDbDriver) (err error) {
	defer func() {
		if err == nil {
			fmt.Println("schemas successfully imported")
		}
	}()
	fmt.Println("-- Importing Schemas --")
	sArgs := defaultScriptArgs()
	scripts := []Script{}
	for i := range driver.GenDbConfig.Schemas {
		script := Script{
			Name: fmt.Sprintf(artifacts.CreateSchemaFileNameFmt, driver.GenDbConfig.Schemas[i]),
		}
		script.Sql, err = artifacts.GetCreateSchemaScript(driver, i)
		if err != nil {
			return err
		}

		scripts = append(scripts, script)
	}

	return runScripts(ctx, driver, sArgs, scripts...)
}

// importUsers uses the CreateUser.sqltemplate to create users defined in schemaConfig.gameDb.users
func importUsers(ctx context.Context, driver *mssql.MssqlDbDriver) (err error) {
	defer func() {
		if err == nil {
			fmt.Println("users successfully imported")
		}
	}()
	fmt.Println("-- Importing Users --")
	sArgs := defaultScriptArgs()
	scripts := []Script{}
	for i := range driver.GenDbConfig.Users {
		script := Script{
			Name: fmt.Sprintf(artifacts.CreateUserFileNameFmt, driver.GenDbConfig.Users[i].Name),
		}
		script.Sql, err = artifacts.GetCreateUserScript(driver, i)
		if err != nil {
			return err
		}

		scripts = append(scripts, script)
	}

	return runScripts(ctx, driver, sArgs, scripts...)
}

// importLogins uses the CreateLogin.sqltemplate to create logins defined in schemaConfig.gameDb.logins
func importLogins(ctx context.Context, driver *mssql.MssqlDbDriver) (err error) {
	defer func() {
		if err == nil {
			fmt.Println("logins successfully imported")
		}
	}()
	fmt.Println("-- Importing Logins --")
	sArgs := defaultScriptArgs()
	sArgs.IsUseDefaultSystemDb = true
	scripts := []Script{}
	for i := range driver.GenDbConfig.Logins {
		script := Script{
			Name: fmt.Sprintf(artifacts.CreateLoginFileNameFmt, driver.GenDbConfig.Logins[i].Name),
		}
		script.Sql, err = artifacts.GetCreateLoginScript(driver, i)
		if err != nil {
			return err
		}

		scripts = append(scripts, script)
	}

	return runScripts(ctx, driver, sArgs, scripts...)
}

// importTables uses the openko-gorm model library to run CREATE TABLE sql scripts, then
// inserts the table data defined in OpenKO-db/ManualSetup/6_InsertData_*.sql
func importTables(ctx context.Context, driver *mssql.MssqlDbDriver) (err error) {
	fmt.Println("-- Creating Tables --")

	scripts, err := getSqlScriptsByPattern(filepath.Join(config.GetConfig().GenConfig.SchemaDir, artifacts.ManualSetupDir), fmt.Sprintf(artifacts.CreateTableFileNameFmt, "*"))
	if err != nil {
		return err
	}

	for i := range scripts {
		scripts[i].Sql = utils.ReplaceUseDatabaseName(scripts[i].Sql, driver.GenDbConfig.Name)
	}

	err = runScripts(ctx, driver, defaultScriptArgs(), scripts...)
	if err != nil {
		return err
	}

	fmt.Println("table structures successfully created")

	fmt.Println("-- Importing Table Data --")
	fmt.Println("this may take several minutes")
	start := time.Now()
	args := defaultScriptArgs()
	args.IsDataDump = true
	scripts, err = getSqlScriptsByPattern(filepath.Join(config.GetConfig().GenConfig.SchemaDir, artifacts.ManualSetupDir), fmt.Sprintf(artifacts.CreateTableDataFileNameFmt, "*"))
	if err != nil {
		return err
	}

	err = runScripts(ctx, driver, args, scripts...)
	if err != nil {
		return err
	}

	fmt.Printf("table data successfully imported in %.2f seconds; batch size %d\n", time.Since(start).Seconds(), ImportBatSize)
	return nil
}

// importViews executes the *.sql scripts in OpenKO-db/Views
func importViews(ctx context.Context, driver *mssql.MssqlDbDriver) (err error) {
	defer func() {
		if err == nil {
			fmt.Println("views successfully imported")
		}
	}()
	fmt.Println("-- Importing Views --")
	scripts, err := getSqlScriptsByPattern(filepath.Join(config.GetConfig().GenConfig.SchemaDir, artifacts.ManualSetupDir), fmt.Sprintf(artifacts.CreateViewFileNameFmt, "*"))
	if err != nil {
		return err
	}

	return runScripts(ctx, driver, defaultScriptArgs(), scripts...)
}

// importViews executes the *.sql scripts in OpenKO-db/StoredProcedures
func importStoredProcs(ctx context.Context, driver *mssql.MssqlDbDriver) (err error) {
	defer func() {
		if err == nil {
			fmt.Println("stored procedures successfully imported")
		}
	}()
	fmt.Println("-- Importing Stored Procedures --")
	scripts, err := getSqlScriptsByPattern(filepath.Join(config.GetConfig().GenConfig.SchemaDir, artifacts.ManualSetupDir), fmt.Sprintf(artifacts.CreateStoredProcedureFileNameFmt, "*"))
	if err != nil {
		return err
	}

	sArgs := defaultScriptArgs()
	return runScripts(ctx, driver, sArgs, scripts...)
}

// getSqlScriptsByPattern returns the list of files from a directory matching the given pattern
func getSqlScriptsByPattern(dir string, pattern string) (sqlScripts []Script, err error) {
	if _, err = os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory %s does not exist", dir)
	}
	fileNames, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return nil, err
	}

	for i := range fileNames {
		//fmt.Printf("Reading %s\n", fileNames[i])
		sqlBytes, err := os.ReadFile(fileNames[i])
		if err != nil {
			return nil, err
		}
		script := Script{
			Name: fileNames[i],
			Sql:  string(sqlBytes),
		}
		sqlScripts = append(sqlScripts, script)
	}

	return sqlScripts, nil
}

// splitBatches breaks an MSSQL .sql dump file into batch groups.  MSSQL dump files use "GO" statements to separate
// batches.  The GO statement is not standard SQL and is only supported inside of MS SQL Management Studio, so we
// need to parse around it.
func splitBatches(sql string) (batches []string) {
	batches = strings.Split(sql, mssql.BatchTerminator)
	for i := range batches {
		batches[i] = strings.TrimSpace(batches[i])
		if batches[i] == "" {
			batches = append(batches[:i], batches[i+1:]...)
		}
	}
	return batches
}

// isIgnoreErr checks an error to see if it can be ignored; These are errors related to
// failed DROP statements after a database clean or new setup
func isIgnoreErr(err error) bool {
	if strings.HasPrefix(err.Error(), "mssql: Cannot drop the view") ||
		strings.HasPrefix(err.Error(), "mssql: Cannot drop the procedure") {
		return true
	}
	return false
}
