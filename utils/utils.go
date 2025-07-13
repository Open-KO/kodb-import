package utils

import "regexp"

// Replaces any USE [dbName] or USE dbName statement with the new database name.
func ReplaceUseDatabaseName(sql, newDbName string) string {
	// 1. Optional leading whitespace
	// 2. USE
	// 3. Optional bracketed or unbracketed DB name (exported code will be bracketed)
	re := regexp.MustCompile(`(?i)^\s*USE\s+(\[)?([^\]\s;]+)(\])?`)

	return re.ReplaceAllStringFunc(sql, func(match string) string {
		submatch := re.FindStringSubmatch(match)
		openingBracket := submatch[1]

		// Preserve original formatting
		replacement := "USE "
		if openingBracket != "" {
			replacement += "[" + newDbName + "]"
		} else {
			replacement += newDbName
		}
		return replacement
	})
}
