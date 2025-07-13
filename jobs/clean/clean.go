package clean

import (
	"context"
	"fmt"
	"kodb-import/mssql"
	"strings"
)

const (
	dropUserSqlFmt = "DROP LOGIN [%s]"
	dropDbSqlFmt   = "DROP DATABASE IF EXISTS [%s]"
)

// Clean will remove any existing [schemaConfig.gameDb.name] database and [schemaConfig.gameDb.users] from an mssql instance
func Clean(ctx context.Context, driver *mssql.MssqlDbDriver) (err error) {
	fmt.Println("-- Clean --")
	conn, err := driver.GetMasterConnection()
	if err != nil {
		return err
	}

	fmt.Printf("Dropping %s database... ", driver.GenDbConfig.Name)
	err = conn.Exec(fmt.Sprintf(dropDbSqlFmt, driver.GenDbConfig.Name)).Error
	if err != nil {
		return err
	}
	fmt.Println(" Done")

	// If the users we're about to create exist in the system database, drop them
	for _, user := range driver.GenDbConfig.Users {
		fmt.Printf("Dropping user %s... ", user.Name)
		err = conn.Exec(fmt.Sprintf(dropUserSqlFmt, user.Name)).Error
		if err != nil {
			// ignore failed drop error - user may not exist.
			if !strings.HasPrefix(err.Error(), "mssql: Cannot drop the login") {
				return err
			}
			fmt.Print(" Not found.")
			err = nil
		}
		fmt.Println(" Done")
	}

	return err
}
