package fees

import (
	"context"
	"database/sql"
	"fmt"

	"encore.dev"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"gocanto.sh/bank/internal/fees/billstore"
	"gocanto.sh/bank/internal/fees/workflows"
	"gocanto.sh/bank/internal/platform/sqlite"
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

	db, err := sqlite.Open(".", cfg.SQLitePath())

	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	c, err := client.Dial(client.Options{HostPort: cfg.TemporalHostPort()})

	if err != nil {
		db.Close()

		return nil, fmt.Errorf("create temporal client: %w", err)
	}

	w := worker.New(c, taskQueue(), worker.Options{})
	w.RegisterWorkflowWithOptions(workflows.Bill, workflow.RegisterOptions{Name: workflows.WorkflowNameBill})

	if err := w.Start(); err != nil {
		c.Close()
		db.Close()

		return nil, fmt.Errorf("start temporal worker: %w", err)
	}

	return &Service{client: c, worker: w, db: db, store: billstore.New(db)}, nil
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
