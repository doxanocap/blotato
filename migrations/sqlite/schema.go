// Package sqlite contains the SQLite schema used at startup.
package sqlite

import _ "embed"

//go:embed 001_init.sql
var Schema string
