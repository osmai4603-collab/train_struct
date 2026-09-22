package examples

import (
	"context"
	"errors"
	"fmt"

	platformerr "train/internal/platform/errors"
)

// ExampleErrorConstruction demonstrates the different ways to create and wrap errors
func ExampleErrorConstruction(ctx context.Context) {
	const op = "examples.ProcessPayment"

	// 1. Using the direct custom constructors
	errNotFound := platformerr.NotFound(op, "wallet not found", nil)
	fmt.Println("Direct NotFound:", errNotFound)

	// 2. Using the flexible E constructor while wrapping an underlying error
	dbErr := errors.New("connection reset")
	errE := platformerr.E(op, platformerr.CodeTimeout, "payment gateway timed out", dbErr)
	fmt.Println("Flexible E:", errE)

	// 3. Enriching the error with the request ID from the context
	enrichedErr := platformerr.FromContext(ctx, errE)
	fmt.Println("Enriched with Context:", enrichedErr)
}
