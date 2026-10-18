package database

// The template's database package supports these SQL drivers. Management and
// HUB can share this package without each service having to register drivers.
import (
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)
