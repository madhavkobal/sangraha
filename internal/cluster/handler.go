package cluster

import (
	"encoding/json"
	"net/http"
)

// statusResponse is the body for GET /admin/v1/cluster/status.
type statusResponse struct {
	NodeID string   `json:"node_id"`
	Role   NodeRole `json:"role"`
	Leader string   `json:"leader"`
	Term   uint64   `json:"term"`
	Peers  []string `json:"peers"`
}

// HandleStatus returns the current cluster status.
func HandleStatus(n *Node) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		peers := make([]string, 0, len(n.peers))
		for _, p := range n.peers {
			peers = append(peers, p.addr)
		}
		resp := statusResponse{
			NodeID: n.cfg.NodeID,
			Role:   n.Role(),
			Leader: n.Leader(),
			Term:   n.Term(),
			Peers:  peers,
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// HandleHeartbeat processes an incoming heartbeat from the leader.
func HandleHeartbeat(n *Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req heartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		if err := n.ReceiveHeartbeat(req.Term, req.LeaderAddr); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// HandleVote processes an incoming vote request from a candidate.
func HandleVote(n *Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req voteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		n.mu.RLock()
		currentTerm := n.term
		n.mu.RUnlock()

		granted := req.Term > currentTerm
		if granted {
			n.mu.Lock()
			n.term = req.Term
			n.role = RoleFollower
			n.mu.Unlock()
		}
		writeJSON(w, http.StatusOK, voteResponse{Granted: granted})
	}
}

// writeJSON marshals v as JSON and writes it to w with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
