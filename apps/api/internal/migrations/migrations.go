// Package migrations embeds the SQL migration files for the application
// database and exposes them for use by the migration tool.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
