//go:build e2e

package fees

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"gocanto.sh/bank/internal/fees/billstore"
	"gocanto.sh/bank/internal/fees/domain"
	"gocanto.sh/bank/internal/fees/workflows"
	"gocanto.sh/bank/internal/platform/sqlite"
	_ "modernc.org/sqlite"
)

func TestServiceE2E_PersistsBillStateInMemorySQLite(t *testing.T) {
	ctx := context.Background()
	service := newE2EService(t, ctx)

	start := time.Now().UTC()
	created, err := service.Create(ctx, &domain.CreateBill{
		BillID:      "service-e2e-bill",
		PeriodStart: start,
		PeriodEnd:   start.Add(time.Hour),
	})

	if err != nil {
		t.Fatalf("create bill: %v", err)
	}

	if created.Data.State != domain.StateOpen {
		t.Fatalf("created state = %q, want open", created.Data.State)
	}

	amount, err := domain.NewMoney(1450, "USD")

	if err != nil {
		t.Fatalf("new money: %v", err)
	}

	updated, err := service.AddLineItem(ctx, "service-e2e-bill", &domain.AddLineItem{
		ID:          "li-service-e2e",
		Description: "Service e2e fee",
		Amount:      amount,
	})

	if err != nil {
		t.Fatalf("add line item: %v", err)
	}

	if len(updated.Data.LineItems) != 1 {
		t.Fatalf("line items = %d, want 1", len(updated.Data.LineItems))
	}

	closed, err := service.Close(ctx, "service-e2e-bill")

	if err != nil {
		t.Fatalf("close bill: %v", err)
	}

	if closed.Data.State != domain.StateClosed {
		t.Fatalf("closed state = %q, want closed", closed.Data.State)
	}

	read, err := service.Get(ctx, "service-e2e-bill")

	if err != nil {
		t.Fatalf("get bill: %v", err)
	}

	if read.Data.State != domain.StateClosed {
		t.Fatalf("read state = %q, want closed", read.Data.State)
	}

	persisted, err := service.store.Find(ctx, "service-e2e-bill")

	if err != nil {
		t.Fatalf("find persisted bill: %v", err)
	}

	if persisted.Totals[0].Amount != 1450 || persisted.Totals[0].Currency != "USD" {
		t.Fatalf("unexpected persisted totals: %#v", persisted.Totals)
	}
}

func newE2EService(t *testing.T, ctx context.Context) *Service {
	t.Helper()

	c := startE2ETemporal(t, ctx)
	w := worker.New(c, taskQueue(), worker.Options{})
	w.RegisterWorkflowWithOptions(workflows.Bill, workflow.RegisterOptions{Name: workflows.WorkflowNameBill})

	if err := w.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}

	db := openE2EMemorySQLite(t)
	service := &Service{
		client: c,
		worker: w,
		db:     db,
		store:  billstore.New(db),
	}

	t.Cleanup(func() {
		service.Shutdown(ctx)
	})

	return service
}

func startE2ETemporal(t *testing.T, ctx context.Context) client.Client {
	t.Helper()

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

	c, err := client.Dial(client.Options{HostPort: host + ":" + port.Port()})

	if err != nil {
		t.Fatalf("dial temporal: %v", err)
	}

	return c
}

func openE2EMemorySQLite(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", "file:gocanto-service-e2e?mode=memory&cache=shared")

	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}

	if err := sqlite.Migrate(db); err != nil {
		db.Close()
		t.Fatalf("migrate in-memory sqlite: %v", err)
	}

	return db
}
