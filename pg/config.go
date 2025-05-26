package pg

import (
	"fmt"
	"time"
)

// Config defines the configuration options for PostgreSQL connections.
type Config struct {
	// Debug enables SQL query logging when set to true.
	Debug bool `yaml:"debug" default:"false"`

	// Host specifies the PostgreSQL server hostname or IP address.
	Host string `yaml:"host"     validate:"required"`
	// Port specifies the PostgreSQL server port number.
	Port int `yaml:"port"     validate:"required"`
	// User specifies the database user name.
	User string `yaml:"user"     validate:"required"`
	// Password specifies the database user password.
	Password string `yaml:"password" validate:"required"`
	// Database specifies the database name to connect to.
	Database string `yaml:"database" validate:"required"`

	// SSLMode specifies the SSL mode for the connection.
	// Valid values: disable, allow, prefer, require, verify-ca, verify-full.
	SSLMode string `yaml:"sslmode"         default:"disable" validate:"oneof=disable allow prefer require verify-ca verify-full"`
	// SearchPath specifies the schema search path.
	SearchPath string `yaml:"search_path"     default:"public"`
	// ConnectTimeout specifies the maximum time to wait when connecting to the server.
	ConnectTimeout time.Duration `yaml:"connect_timeout" default:"10s"`

	// PoolMaxConns specifies the maximum number of connections in the pool.
	PoolMaxConns int32 `yaml:"pool_max_conns"          default:"4"`
	// PoolMinConns specifies the minimum number of connections in the pool.
	PoolMinConns int32 `yaml:"pool_min_conns"          default:"1"`
	// PoolMaxConnLifetime specifies the maximum lifetime of a connection.
	PoolMaxConnLifetime time.Duration `yaml:"pool_max_conn_lifetime"  default:"1h"`
	// PoolMaxConnIdleTime specifies how long a connection can remain idle in the pool.
	PoolMaxConnIdleTime time.Duration `yaml:"pool_max_conn_idle_time" default:"30m"`
}

// dsn returns a PostgreSQL connection string built from the configuration.
func (c Config) dsn() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s search_path=%s connect_timeout=%d",
		c.Host,
		c.Port,
		c.User,
		c.Password,
		c.Database,
		c.SSLMode,
		c.SearchPath,
		int(c.ConnectTimeout.Seconds()),
	)
}
