package examples

import (
	"context"
)

// RequestMetadata bundles all common request attributes into a single struct to avoid an O(N) tree
type RequestMetadata struct {
	RequestID     string
	CorrelationID string
	TenantID      string
	UserID        string
	ClientIP      string
}

type metadataKey struct{}

// WithRequestMetadata injects the metadata object in one go into a single context node
func WithRequestMetadata(ctx context.Context, meta *RequestMetadata) context.Context {
	if meta == nil {
		return ctx
	}
	return context.WithValue(ctx, metadataKey{}, meta)
}

// MetadataFromContext retrieves the metadata object
func MetadataFromContext(ctx context.Context) (*RequestMetadata, bool) {
	meta, ok := ctx.Value(metadataKey{}).(*RequestMetadata)
	return meta, ok
}

// RequestIDFromContext is a fast helper for extracting the request ID
func RequestIDFromContext(ctx context.Context) (string, bool) {
	if meta, ok := MetadataFromContext(ctx); ok && meta.RequestID != "" {
		return meta.RequestID, true
	}
	return "", false
}
