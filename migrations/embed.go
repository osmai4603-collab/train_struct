// Package migrations embeds the SQL schema migration files into the binary so
// the server and tooling can apply them without external seed files.
package migrations

import "embed"

// FS holds all *.sql files under the migrations directory.
//
//go:embed *.sql
var FS embed.FS
