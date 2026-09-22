package context

import (
	stdctx "context"
)

// RequestMetadata collects common request attributes into a single structure to avoid an O(N) tree depth
type RequestMetadata struct {
	RequestID     string `json:"request_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	TenantID      string `json:"tenant_id,omitempty"`
	UserID        string `json:"user_id,omitempty"`
	ClientIP      string `json:"client_ip,omitempty"`
}

// metadataKey is an unexported zero-byte size type to prevent any key collisions
type metadataKey struct{}

// WithRequestMetadata injects the request metadata object at once into a single context node
func WithRequestMetadata(ctx stdctx.Context, meta *RequestMetadata) stdctx.Context {
	if meta == nil {
		return ctx
	}
	return stdctx.WithValue(ctx, metadataKey{}, meta)
}

// MetadataFromContext retrieves the request metadata object from the context
func MetadataFromContext(ctx stdctx.Context) (*RequestMetadata, bool) {
	if ctx == nil {
		return nil, false
	}
	meta, ok := ctx.Value(metadataKey{}).(*RequestMetadata)
	return meta, ok
}

// WithRequestID injects the request ID into the existing RequestMetadata object or creates a new one
func WithRequestID(ctx stdctx.Context, requestID string) stdctx.Context {
	meta, ok := MetadataFromContext(ctx)
	if !ok {
		meta = &RequestMetadata{}
	} else {
		// Create a new copy to preserve immutability
		shallowCopy := *meta
		meta = &shallowCopy
	}
	meta.RequestID = requestID
	return WithRequestMetadata(ctx, meta)
}

// RequestIDFromContext retrieves the request ID if present
func RequestIDFromContext(ctx stdctx.Context) (string, bool) {
	if meta, ok := MetadataFromContext(ctx); ok && meta.RequestID != "" {
		return meta.RequestID, true
	}
	return "", false
}

// WithTenantID injects the tenant ID into the RequestMetadata object
func WithTenantID(ctx stdctx.Context, tenantID string) stdctx.Context {
	meta, ok := MetadataFromContext(ctx)
	if !ok {
		meta = &RequestMetadata{}
	} else {
		shallowCopy := *meta
		meta = &shallowCopy
	}
	meta.TenantID = tenantID
	return WithRequestMetadata(ctx, meta)
}

// TenantIDFromContext retrieves the tenant ID for multi-tenant data isolation
func TenantIDFromContext(ctx stdctx.Context) (string, bool) {
	if meta, ok := MetadataFromContext(ctx); ok && meta.TenantID != "" {
		return meta.TenantID, true
	}
	return "", false
}

// WithUserID injects the user ID
func WithUserID(ctx stdctx.Context, userID string) stdctx.Context {
	meta, ok := MetadataFromContext(ctx)
	if !ok {
		meta = &RequestMetadata{}
	} else {
		shallowCopy := *meta
		meta = &shallowCopy
	}
	meta.UserID = userID
	return WithRequestMetadata(ctx, meta)
}

// UserIDFromContext retrieves the user ID
func UserIDFromContext(ctx stdctx.Context) (string, bool) {
	if meta, ok := MetadataFromContext(ctx); ok && meta.UserID != "" {
		return meta.UserID, true
	}
	return "", false
}
