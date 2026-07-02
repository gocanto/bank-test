package fees

import (
	"context"
	"database/sql"

	"encore.dev"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"gocanto.sh/bank/internal/bootstrap"
	"gocanto.sh/bank/internal/fees/billstore"
	"gocanto.sh/bank/internal/fees/workflows"
)

//encore:service
type Service struct {
	client client.Client
	worker worker.Worker
	db     *sql.DB
	store  *billstore.Store
}

func initService() (*Service, error) {
	cfg := appConfig()

	deps, err := bootstrap.New(bootstrap.Config{
		SQLitePath:       cfg.SQLitePath(),
		TemporalHostPort: cfg.TemporalHostPort(),
		TaskQueue:        taskQueue(),
	})

	if err != nil {
		return nil, err
	}

	return &Service{client: deps.Client, worker: deps.Worker, db: deps.DB, store: deps.Store}, nil
}

func (s *Service) Shutdown(force context.Context) {
	if s == nil {
		return
	}

	if s.worker != nil {
		s.worker.Stop()
	}

	if s.client != nil {
		s.client.Close()
	}

	if s.db != nil {
		_ = s.db.Close()
	}
}

func taskQueue() string {
	envName := encoreEnvironmentName()

	if envName == "" {
		return workflows.DefaultTaskQueue
	}

	return envName + "-" + workflows.DefaultTaskQueue
}

func encoreEnvironmentName() (name string) {
	defer func() {
		if recover() != nil {
			name = ""
		}
	}()

	return encore.Meta().Environment.Name
}
