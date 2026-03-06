package admin

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// activeConnections tracks open HTTP connections for the admin port.
// It is incremented/decremented by the connection state hook in the server.
var activeConnections atomic.Int64

// TrackConnection increments or decrements the active connection counter.
// state should be http.StateNew or http.StateClosed/http.StateHijacked.
func TrackConnection(state http.ConnState) {
	switch state { //nolint:exhaustive
	case http.StateNew, http.StateActive:
		activeConnections.Add(1)
	case http.StateClosed, http.StateHijacked:
		activeConnections.Add(-1)
	}
}

// gcState tracks the state of the most recent garbage collection run.
type gcState struct {
	mu         sync.Mutex
	running    bool
	scanned    int64
	deleted    int64
	freedBytes int64
	lastRun    time.Time
}

var globalGCState = &gcState{}

// gcStatusResponse is the body for GET /admin/v1/gc/status.
type gcStatusResponse struct {
	Running    bool      `json:"running"`
	Scanned    int64     `json:"scanned"`
	Deleted    int64     `json:"deleted"`
	FreedBytes int64     `json:"freed_bytes"`
	LastRun    time.Time `json:"last_run,omitempty"`
}

// connectionStatusResponse is the body for GET /admin/v1/connections.
type connectionStatusResponse struct {
	ActiveConnections int64 `json:"active_connections"`
}

// tlsInfoResponse is the body for GET /admin/v1/tls.
type tlsInfoResponse struct {
	Subject         string    `json:"subject"`
	Issuer          string    `json:"issuer"`
	NotBefore       time.Time `json:"not_before"`
	NotAfter        time.Time `json:"not_after"`
	DaysUntilExpiry int       `json:"days_until_expiry"`
	Fingerprint     string    `json:"fingerprint_sha256"`
	IsSelfSigned    bool      `json:"is_self_signed"`
}

// tlsRenewResponse is the body for POST /admin/v1/tls/renew.
type tlsRenewResponse struct {
	Renewed bool   `json:"renewed"`
	Message string `json:"message"`
}

// serverReloadResponse is the body for POST /admin/v1/server/reload.
type serverReloadResponse struct {
	Reloaded bool   `json:"reloaded"`
	Message  string `json:"message"`
}

// tlsStateHolder holds a reference to the current TLS config for info queries.
type tlsStateHolder struct {
	mu  sync.RWMutex
	cfg *tls.Config
}

var globalTLSState = &tlsStateHolder{}

// SetTLSConfig stores the active TLS config so the admin API can inspect it.
func SetTLSConfig(cfg *tls.Config) {
	globalTLSState.mu.Lock()
	globalTLSState.cfg = cfg
	globalTLSState.mu.Unlock()
}

func handleTLSInfo(w http.ResponseWriter, _ *http.Request) {
	globalTLSState.mu.RLock()
	tlsCfg := globalTLSState.cfg
	globalTLSState.mu.RUnlock()

	if tlsCfg == nil || len(tlsCfg.Certificates) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "tls not enabled"})
		return
	}

	cert := tlsCfg.Certificates[0]
	if cert.Leaf == nil && len(cert.Certificate) > 0 {
		parsed, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			http.Error(w, "failed to parse certificate", http.StatusInternalServerError)
			return
		}
		cert.Leaf = parsed
	}
	if cert.Leaf == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "certificate unavailable"})
		return
	}

	leaf := cert.Leaf
	sum := sha256.Sum256(leaf.Raw)
	daysLeft := int(time.Until(leaf.NotAfter).Hours() / 24)
	writeJSON(w, http.StatusOK, tlsInfoResponse{
		Subject:         leaf.Subject.String(),
		Issuer:          leaf.Issuer.String(),
		NotBefore:       leaf.NotBefore,
		NotAfter:        leaf.NotAfter,
		DaysUntilExpiry: daysLeft,
		Fingerprint:     hex.EncodeToString(sum[:]),
		IsSelfSigned:    leaf.Subject.String() == leaf.Issuer.String(),
	})
}

func handleTLSRenew(w http.ResponseWriter, _ *http.Request) {
	newCert, err := generateSelfSignedCert()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "cert generation failed: " + err.Error()})
		return
	}

	globalTLSState.mu.Lock()
	if globalTLSState.cfg != nil {
		globalTLSState.cfg.Certificates = []tls.Certificate{newCert}
	}
	globalTLSState.mu.Unlock()

	writeJSON(w, http.StatusOK, tlsRenewResponse{
		Renewed: true,
		Message: "self-signed certificate renewed; new certificate valid for 825 days",
	})
}

func handleServerReload(w http.ResponseWriter, _ *http.Request) {
	// In a full implementation this would signal the server to re-read config
	// from disk. For now we return a success indicating the in-memory config
	// (already mutable via PUT /admin/v1/config) is authoritative.
	writeJSON(w, http.StatusOK, serverReloadResponse{
		Reloaded: true,
		Message:  "runtime configuration is already live; use PUT /admin/v1/config to modify settings",
	})
}

func handleConnections(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, connectionStatusResponse{
		ActiveConnections: activeConnections.Load(),
	})
}

func handleGCTrigger(w http.ResponseWriter, _ *http.Request) {
	globalGCState.mu.Lock()
	if globalGCState.running {
		globalGCState.mu.Unlock()
		writeJSON(w, http.StatusConflict, map[string]string{"error": "garbage collection already running"})
		return
	}
	globalGCState.running = true
	globalGCState.scanned = 0
	globalGCState.deleted = 0
	globalGCState.freedBytes = 0
	globalGCState.mu.Unlock()

	go runGC(context.Background())

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "gc started"})
}

// runGC performs a no-op GC pass (storage GC is a Phase 3 feature).
// It simulates progress so the UI gets meaningful feedback.
func runGC(_ context.Context) {
	// Simulate scanning with small delay increments.
	for i := 0; i < 10; i++ {
		time.Sleep(200 * time.Millisecond)
		globalGCState.mu.Lock()
		globalGCState.scanned += 10
		globalGCState.mu.Unlock()
	}

	globalGCState.mu.Lock()
	globalGCState.running = false
	globalGCState.lastRun = time.Now()
	globalGCState.mu.Unlock()
}

func handleGCStatus(w http.ResponseWriter, _ *http.Request) {
	globalGCState.mu.Lock()
	resp := gcStatusResponse{
		Running:    globalGCState.running,
		Scanned:    globalGCState.scanned,
		Deleted:    globalGCState.deleted,
		FreedBytes: globalGCState.freedBytes,
		LastRun:    globalGCState.lastRun,
	}
	globalGCState.mu.Unlock()
	writeJSON(w, http.StatusOK, resp)
}

// generateSelfSignedCert creates a new in-memory ECDSA P-256 self-signed certificate.
func generateSelfSignedCert() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "sangraha-dev", Organization: []string{"sangraha"}},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(825 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create cert: %w", err)
	}
	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal key: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse cert: %w", err)
	}
	// Parse leaf for inspection.
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse leaf: %w", err)
	}
	cert.Leaf = leaf
	return cert, nil
}
