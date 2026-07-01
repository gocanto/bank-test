package fees

import (
	"context"

	"go.temporal.io/sdk/client"
)

func query[T any](ctx context.Context, c client.Client, workflowID string, queryName string) (T, error) {
	var zero T

	response, err := c.QueryWorkflow(ctx, workflowID, "", queryName)

	if err != nil {
		var result T
		run := c.GetWorkflow(ctx, workflowID, "")

		if getErr := run.Get(ctx, &result); getErr == nil {
			return result, nil
		}

		return zero, err
	}

	var result T

	if err := response.Get(&result); err != nil {
		return zero, err
	}

	return result, nil
}
