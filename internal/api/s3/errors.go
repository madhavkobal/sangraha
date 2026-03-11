// Package s3 implements the S3-compatible HTTP API.
package s3

import (
	"encoding/xml"
	"net/http"

	"github.com/madhavkobal/sangraha/pkg/s3types"
)

// writeError writes an S3-compatible XML error response.
func writeError(w http.ResponseWriter, r *http.Request, statusCode int, code, message string) {
	reqID := requestIDFromContext(r.Context())
	resp := s3types.ErrorResponse{
		Code:      code,
		Message:   message,
		Resource:  r.URL.Path,
		RequestID: reqID,
	}
	body, err := xml.Marshal(resp)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-amz-request-id", reqID)
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(body) //nolint:gosec // G705: body is xml.Marshal output (XML-escaped); Content-Type is application/xml
}

// writeXML serialises v to XML and writes it with status code.
func writeXML(w http.ResponseWriter, r *http.Request, statusCode int, v any) {
	reqID := requestIDFromContext(r.Context())
	body, err := xml.Marshal(v)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "InternalError", "failed to serialise response")
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("x-amz-request-id", reqID)
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(body) //nolint:gosec // G705: body is xml.Marshal output (XML-escaped); Content-Type is application/xml
}
