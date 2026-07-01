package fees

import "encore.dev/config"

type Config struct {
	TemporalHostPort config.String
	SQLitePath       config.String
}

func appConfig() *Config {
	return config.Load[*Config]()
}
