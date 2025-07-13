package arg

import (
	"flag"
	"fmt"
	"kodb-import/config"
)

// Args defines and handles the CLI input flags/arguments
type Args struct {
	Clean           bool
	Import          bool
	ImportBatchSize int
	ConfigPath      string
	DbUser          string
	DbPass          string
	SchemaDir       string
}

// Validate ensures that the combination of arguments used is valid
func (this Args) Validate() (err error) {
	if !(this.Clean || this.Import) {
		flag.Usage()
		return fmt.Errorf("no actionable arguments provided")
	}

	return nil
}

// GetArgs reads the CLI arguments using the go flag package
func GetArgs() (a Args) {
	_clean := flag.Bool("clean", false, "Clean drops any configured users and drops the databaseConfig.dbname database")
	_import := flag.Bool("import", false, "Runs clean and imports the contents of OpenKO-db/ManualSetup, StoredProcedures, and Views")
	importBatchSize := flag.Int("batchSize", 16, "Batch sized used when importing table data.  Valid range [2-999], if invalid value specified will default to 16")
	configPath := flag.String("config", config.DefaultConfigFileName, "Path to config file, inclusive of the filename")
	dbUser := flag.String("dbuser", "", "Database connection user override")
	dbPass := flag.String("dbpass", "", "Database connection password override")
	schemaDir := flag.String("schema", "", "OpenKO-db schema directory override; in most cases you'll just want to use the default git submodule location")

	flag.Parse()

	if _clean != nil {
		a.Clean = *_clean
	}

	if _import != nil {
		a.Import = *_import
	}

	if configPath != nil {
		a.ConfigPath = *configPath
		config.ConfigPath = *configPath
	}

	if dbUser != nil {
		a.DbUser = *dbUser
	}

	if dbPass != nil {
		a.DbPass = *dbPass
	}

	if schemaDir != nil {
		a.SchemaDir = *schemaDir
	}

	if importBatchSize != nil {
		a.ImportBatchSize = *importBatchSize
	}

	return a
}
