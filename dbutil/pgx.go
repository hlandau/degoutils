package dbutil

import "github.com/jackc/pgx"
import "strings"
import "gopkg.in/hlandau/easyconfig.v1/cflag"

var maxConnectionsFlag = cflag.Int(nil, "maxpgconnections", 2, "Maximum number of PostgreSQL pool connections")

func NewPgxPool(url string) (*pgx.ConnPool, error) {
	dbcfg := pgx.ConnPoolConfig{
		MaxConnections: maxConnectionsFlag.Value(),
	}

	var err error
	if url == "" {
		dbcfg.ConnConfig, err = pgx.ParseEnvLibpq()
	} else if strings.HasPrefix(url, "postgresql://") {
		dbcfg.ConnConfig, err = pgx.ParseURI(url)
	} else {
		dbcfg.ConnConfig, err = pgx.ParseDSN(url)
	}

	if err != nil {
		return nil, err
	}

	return pgx.NewConnPool(dbcfg)
}
