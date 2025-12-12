package rediswr

// Config defines the configuration options for Redis connections.
type Config struct {

	// Addrs is the list of Redis server addresses in the format "host:port,host2:port2".
	Addrs string `yaml:"addrs" validate:"required"`

	// Username is the username for the Redis server/cluster.
	Username string `yaml:"username"`

	// Password is the password for the Redis server/cluster.
	Password string `yaml:"password"`

	// IsClusterMode indicates whether the Redis server is a Redis cluster.
	IsClusterMode bool `yaml:"is_cluster_mode"`
}
