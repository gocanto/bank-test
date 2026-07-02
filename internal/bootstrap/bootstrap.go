// Package bootstrap performs the framework-free wiring for the fees service:
// opening SQLite, dialing Temporal, and starting the worker with the bill
// workflow registered. It carries no Encore annotations so the service package
// stays a thin adapter over these dependencies.
package bootstrap

import (
	"database/sql"
	"fmt"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"gocanto.sh/bank/internal/fees/billstore"
	"gocanto.sh/bank/internal/fees/workflows"
	"gocanto.sh/bank/internal/platform/sqlite"
)

type Config struct {
	SQLitePath       string
	TemporalHostPort string
	TaskQueue        string
}

type Deps struct {
	Client client.Client
	Worker worker.Worker
	DB     *sql.DB
	Store  *billstore.Store
}

// New wires the service dependencies, cleaning up any partially-initialised
// resources if a later step fails.
func New(cfg Config) (*Deps, error) {
	db, err := sqlite.Open(".", cfg.SQLitePath)

	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	c, err := client.Dial(client.Options{HostPort: cfg.TemporalHostPort})

	if err != nil {
		db.Close()

		return nil, fmt.Errorf("create temporal client: %w", err)
	}

	w := worker.New(c, cfg.TaskQueue, worker.Options{})
	w.RegisterWorkflowWithOptions(workflows.Bill, workflow.RegisterOptions{Name: workflows.WorkflowNameBill})

	if err := w.Start(); err != nil {
		c.Close()
		db.Close()

		return nil, fmt.Errorf("start temporal worker: %w", err)
	}

	return &Deps{Client: c, Worker: w, DB: db, Store: billstore.New(db)}, nil
}
