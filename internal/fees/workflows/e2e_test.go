//go:build e2e

package workflows_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"gocanto.sh/bank/internal/fees/domain"
	"gocanto.sh/bank/internal/fees/workflows"
)

func TestTemporalE2E_CreateAddCloseBill(t *testing.T) {
	ctx, c := startTemporal(t)

	start := time.Now().UTC()
	billReq := domain.CreateBill{BillID: "e2e-bill", PeriodStart: start, PeriodEnd: start.Add(time.Hour)}
	run, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        workflows.WorkflowID(billReq.BillID),
		TaskQueue: workflows.DefaultTaskQueue,
	}, workflows.WorkflowNameBill, billReq)

	if err != nil {
		t.Fatalf("execute workflow: %v", err)
	}

	amount, err := domain.NewMoney(1200, "USD")

	if err != nil {
		t.Fatalf("new money: %v", err)
	}

	update, err := c.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   run.GetID(),
		RunID:        run.GetRunID(),
		UpdateName:   workflows.UpdateAddLineItem,
		Args:         []any{domain.AddLineItem{ID: "li-1", Description: "E2E fee", Amount: amount}},
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})

	if err != nil {
		t.Fatalf("add update: %v", err)
	}

	var summary domain.Bill

	if err := update.Get(ctx, &summary); err != nil {
		t.Fatalf("add update result: %v", err)
	}

	update, err = c.UpdateWorkflow(ctx, client.UpdateWorkflowOptions{
		WorkflowID:   run.GetID(),
		RunID:        run.GetRunID(),
		UpdateName:   workflows.UpdateCloseBill,
		WaitForStage: client.WorkflowUpdateStageCompleted,
	})

	if err != nil {
		t.Fatalf("close update: %v", err)
	}

	if err := update.Get(ctx, &summary); err != nil {
		t.Fatalf("close update result: %v", err)
	}

	if summary.State != domain.StateClosed {
		t.Fatalf("state = %q, want closed", summary.State)
	}
}

func startTemporal(t *testing.T) (context.Context, client.Client) {
	t.Helper()

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "temporalio/temporal:latest",
		ExposedPorts: []string{"7233/tcp"},
		Cmd:          []string{"server", "start-dev", "--ip", "0.0.0.0"},
		WaitingFor:   wait.ForListeningPort("7233/tcp").WithStartupTimeout(90 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		t.Fatalf("start temporal container: %v", err)
	}

	t.Cleanup(func() {
		testcontainers.TerminateContainer(container)
	})

	host, err := container.Host(ctx)

	if err != nil {
		t.Fatalf("container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "7233")

	if err != nil {
		t.Fatalf("container port: %v", err)
	}

	c := dialE2ETemporal(t, host+":"+port.Port())

	t.Cleanup(c.Close)

	w := worker.New(c, workflows.DefaultTaskQueue, worker.Options{})
	w.RegisterWorkflowWithOptions(workflows.Bill, workflow.RegisterOptions{Name: workflows.WorkflowNameBill})

	if err := w.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}

	t.Cleanup(w.Stop)

	return ctx, c
}

// dialE2ETemporal retries the connection until Temporal's gRPC frontend is
// actually serving. A listening port only means the container is up; the
// frontend may still reset the connection mid-handshake while it starts.
func dialE2ETemporal(t *testing.T, hostPort string) client.Client {
	t.Helper()

	deadline := time.Now().Add(60 * time.Second)
	var lastErr error

	for time.Now().Before(deadline) {
		c, err := client.Dial(client.Options{HostPort: hostPort})

		if err != nil {
			lastErr = err
			time.Sleep(time.Second)

			continue
		}

		if _, err := c.CheckHealth(context.Background(), &client.CheckHealthRequest{}); err != nil {
			lastErr = err
			c.Close()
			time.Sleep(time.Second)

			continue
		}

		return c
	}

	t.Fatalf("dial temporal: %v", lastErr)

	return nil
}
