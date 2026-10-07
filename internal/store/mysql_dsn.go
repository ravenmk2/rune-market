package store

import (
	"github.com/go-sql-driver/mysql"

	"github.com/ravenmk2/rune-market/internal/config"
)

// BuildMySQLDSN assembles a DSN from the wizard's structured fields and
// enforces the §6.1 parameters (parseTime, utf8mb4_bin collation; the
// collation pins the utf8mb4 charset).
func BuildMySQLDSN(db config.Database) string {
	cfg := mysql.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = db.Host
	cfg.User = db.Username
	cfg.Passwd = db.Password
	cfg.DBName = db.Database
	cfg.ParseTime = true
	cfg.Collation = "utf8mb4_bin"
	return cfg.FormatDSN()
}
