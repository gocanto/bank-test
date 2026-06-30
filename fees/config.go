package fees

import "encore.dev/config"

type Config struct {
	TemporalHostPort config.String
}

var cfg = config.Load[*Config]()
