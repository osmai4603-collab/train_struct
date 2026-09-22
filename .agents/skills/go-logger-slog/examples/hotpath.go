package examples

import (
	"context"
	"log/slog"
	"time"
)

// OrderData represents the order data structure for the hot path
type OrderData struct {
	ID         string
	Amount     int64
	CustomerID string
}

// ProcessBatchWithHotPathDemonstration demonstrates high-performance best practices in batch processing
func ProcessBatchWithHotPathDemonstration(ctx context.Context, logger *slog.Logger, orders []OrderData) {
	// 1. Use logger.With to pin down shared attributes and avoid maintaining them repeatedly
	batchLogger := logger.With(
		slog.String("process", "batch_settlement"),
		slog.Int("total_items", len(orders)),
	)

	batchLogger.InfoContext(ctx, "starting batch processing")
	start := time.Now()

	var successCount int
	var failCount int

	for _, order := range orders {
		// Simulated processing of the order
		err := simulateProcessing(order)
		if err != nil {
			failCount++
			// 2. Use LogAttrs in repeated paths to avoid heap allocations
			batchLogger.LogAttrs(ctx, slog.LevelError, "order failed in batch",
				slog.String("order_id", order.ID),
				slog.String("customer_id", order.CustomerID),
				slog.String("error", err.Error()),
			)
			continue
		}
		successCount++
	}

	// 3. Check upfront via Enabled before building expensive diagnostic objects
	if batchLogger.Enabled(ctx, slog.LevelDebug) {
		diagnostics := generateExpensiveDiagnosticDump(orders)
		batchLogger.DebugContext(ctx, "batch processing diagnostics",
			slog.Any("diagnostics", diagnostics),
		)
	}

	// 4. Log a single summary when the batch completes instead of flooding logs inside the loop
	batchLogger.LogAttrs(ctx, slog.LevelInfo, "batch processing completed",
		slog.Int("success", successCount),
		slog.Int("failed", failCount),
		slog.Duration("elapsed", time.Since(start)),
	)
}

func simulateProcessing(o OrderData) error {
	_ = o
	return nil
}

func generateExpensiveDiagnosticDump(orders []OrderData) map[string]any {
	return map[string]any{
		"sample_count": len(orders),
		"captured_at":  time.Now().UnixNano(),
	}
}
