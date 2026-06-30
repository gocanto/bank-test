package fees

import (
	"context"
	"fmt"

	"encore.app/fees/workflows"
	"encore.dev"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

var envName = encore.Meta().Environment.Name

//encore:service
type Service struct {
	client client.Client
	worker worker.Worker
}

func initService() (*Service, error) {
	c, err := client.Dial(client.Options{HostPort: cfg.TemporalHostPort()})
	if err != nil {
		return nil, fmt.Errorf("create temporal client: %w", err)
	}

	w := worker.New(c, taskQueue(), worker.Options{})
	w.RegisterWorkflowWithOptions(workflows.Bill, workflow.RegisterOptions{Name: workflows.WorkflowNameBill})

	if err := w.Start(); err != nil {
		c.Close()
		return nil, fmt.Errorf("start temporal worker: %w", err)
	}

	return &Service{client: c, worker: w}, nil
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
}

func taskQueue() string {
	if envName == "" {
		return workflows.DefaultTaskQueue
	}

	return envName + "-" + workflows.DefaultTaskQueue
}
