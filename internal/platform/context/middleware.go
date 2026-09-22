package context

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// Header constants used as a standard in our project
const (
	HeaderRequestID     = "X-Request-ID"
	HeaderCorrelationID = "X-Correlation-ID"
	HeaderTenantID      = "X-Tenant-ID"
)

// generateRandomID generates a secure, fast hexadecimal ID without external dependencies
func generateRandomID(byteLen int) string {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "fallback-id"
	}
	return hex.EncodeToString(b)
}

// Middleware creates an HTTP middleware that extracts request attributes and injects them into the request context
// It also ensures the request ID is returned to the client in the response headers
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(HeaderRequestID)
		if reqID == "" {
			reqID = generateRandomID(16) // 32 hex chars
		}

		corrID := r.Header.Get(HeaderCorrelationID)
		if corrID == "" {
			corrID = reqID
		}

		tenantID := r.Header.Get(HeaderTenantID)

		meta := &RequestMetadata{
			RequestID:     reqID,
			CorrelationID: corrID,
			TenantID:      tenantID,
			ClientIP:      r.RemoteAddr,
		}

		// Inject the attributes into the request context
		ctx := WithRequestMetadata(r.Context(), meta)

		// Write the ID to the response header
		w.Header().Set(HeaderRequestID, reqID)
		w.Header().Set(HeaderCorrelationID, corrID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
