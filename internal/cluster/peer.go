package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// heartbeatRequest is the JSON body for POST /admin/v1/cluster/heartbeat.
type heartbeatRequest struct {
	Term       uint64 `json:"term"`
	LeaderAddr string `json:"leader_addr"`
}

// voteRequest is the JSON body for POST /admin/v1/cluster/vote.
type voteRequest struct {
	Term        uint64 `json:"term"`
	CandidateID string `json:"candidate_id"`
}

// voteResponse is the JSON response for POST /admin/v1/cluster/vote.
type voteResponse struct {
	Granted bool `json:"granted"`
}

// peerClient sends cluster RPC calls to a single peer over HTTP.
type peerClient struct {
	addr       string
	httpClient *http.Client
}

// newPeerClient creates a peerClient targeting the given advertise address.
func newPeerClient(addr string) *peerClient {
	return &peerClient{
		addr: addr,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

// SendHeartbeat sends a heartbeat to the peer's admin endpoint.
func (c *peerClient) SendHeartbeat(ctx context.Context, term uint64, leaderAddr string) error {
	body, err := json.Marshal(heartbeatRequest{Term: term, LeaderAddr: leaderAddr})
	if err != nil {
		return fmt.Errorf("peer heartbeat: marshal: %w", err)
	}
	url := "http://" + c.addr + "/admin/v1/cluster/heartbeat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("peer heartbeat: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("peer heartbeat: send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("peer heartbeat: peer returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// RequestVote asks the peer to grant a vote for the given candidate in the given term.
// Returns true if the peer granted the vote.
func (c *peerClient) RequestVote(ctx context.Context, term uint64, candidateID string) bool {
	body, err := json.Marshal(voteRequest{Term: term, CandidateID: candidateID})
	if err != nil {
		return false
	}
	url := "http://" + c.addr + "/admin/v1/cluster/vote"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var vr voteResponse
	if err = json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		return false
	}
	return vr.Granted
}
