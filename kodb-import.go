package main

import (
	"context"
	"fmt"
	"kodb-import/arg"
	"kodb-import/config"
	"kodb-import/jobs/clean"
	"kodb-import/jobs/importDb"
	"kodb-import/mssql"
	"log"
	"strings"

	"github.com/Open-KO/kodb-godef/enums/dbType"
	"gorm.io/gorm"
)

const (
	appTitle    = "OpenKO Database Import Utility"
	outputWidth = 120
)

type dbInfo struct {
	Type   dbType.DbType
	Config config.GenDbConfig
}

func printHeaderRow() {
	fmt.Printf("%s\n", strings.Repeat("-", outputWidth))
}

func main() {
	defer func() {
		// catch-all panic error
		if r := recover(); r != nil {
			log.Printf("Recovered from panic: %v", r)
		}
	}()

	// Print intro header
	printHeaderRow()
	titlePad := (outputWidth - len(appTitle)) / 2
	fmt.Printf("%[2]s%[1]s%[2]s\n", appTitle, strings.Repeat(" ", titlePad))
	printHeaderRow()

	args := arg.GetArgs()
	if err := args.Validate(); err != nil {
		fmt.Printf("arguments error: %v, closing.", err)
		return
	}

	// loading config for the first time can throw a panic, so let's do it here
	// uses a singleton pattern, so once loaded from disk it's in memory
	fmt.Print("Loading config...")
	conf := config.GetConfig()
	// apply any command-line overrides
	if args.DbUser != "" {
		conf.DatabaseConfig.User = args.DbUser
	}
	if args.DbPass != "" {
		conf.DatabaseConfig.Password = args.DbPass
	}
	if args.SchemaDir != "" {
		conf.GenConfig.SchemaDir = args.SchemaDir
	}
	if args.ImportBatchSize > 1 && args.ImportBatchSize < 1000 {
		importDb.ImportBatSize = args.ImportBatchSize
	}
	fmt.Println("done")

	// Create a stub context for use with our db-ops.  We're not doing anything fancy with it now, but it will give us a
	// few options if we ever desire them (deadlines, cancel funcs, key:val mapping)
	// https://pkg.go.dev/context
	appCtx := context.Background()

	dbs := []dbInfo{}
	for i := range conf.GenConfig.GameDbs {
		dbs = append(dbs, dbInfo{
			Config: conf.GenConfig.GameDbs[i],
			Type:   dbType.GAME,
		})
	}

	// TODO: Add multi-db support by updating the config structure with LoginDbs and LogDbs
	// and adding them to the dbs list

	for i := range dbs {
		err := processDb(appCtx, dbs[i], args)
		if err != nil {
			panic(err)
		}
	}
}

// processDb attempts requested jobs for the given database
func processDb(appCtx context.Context, db dbInfo, args arg.Args) (err error) {
	// a clean driver should be used/configured per database as the application logic
	// makes heavy use of the driver.GenDbConfig
	driver := mssql.NewMssqlDbDriver(db.Config, db.Type)

	var tx *gorm.DB
	defer func() {
		// catch-all panic error
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
		if tx != nil {
			if err != nil {
				rErr := driver.RollbackTx()
				if rErr != nil {
					fmt.Printf("failed to rollback transaction: %v", rErr)
				}
			} else {
				err = driver.CommitTx()
			}
		}
		driver.CloseConnection()
	}()

	// Run clean if either -clean or -import was called
	if args.Clean || args.Import {
		err = clean.Clean(appCtx, driver)
		if err != nil {
			return err
		}
	}

	if args.Import {
		err = importDb.ImportDb(appCtx, driver)
		if err != nil {
			return err
		}
	}

	// ImportDb will set driver.Tx as it has a mix of work to do on master/gen databases.  Get a ref to that pointer,
	// or open it now if import wasn't called
	tx, err = driver.GetTx()
	if err != nil {
		return err
	}

	return nil
}
