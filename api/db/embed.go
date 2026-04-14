package db

import "embed"

// FS embeds all SQL migration scripts in the migrations directory.
//
//go:embed migrations/*.sql
var FS embed.FS
