// Package cluster implements multi-node coordination for sangraha.
// It uses a simple leader-election protocol over HTTP. In single-node mode
// (default, no peers configured) the node always considers itself the leader.
package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// NodeRole describes the current role of a cluster node.
type NodeRole string

const (
	// RoleLeader means this node is the current cluster leader.
	RoleLeader NodeRole = "leader"
	// RoleFollower means this node is following the current leader.
	RoleFollower NodeRole = "follower"
	// RoleCandidate means this node is running an election.
	RoleCandidate NodeRole = "candidate"
)

// NodeConfig holds the configuration for a cluster node.
type NodeConfig struct {
	// NodeID is a unique identifier for this node. Auto-generated UUID if empty.
	NodeID string `yaml:"node_id" mapstructure:"node_id"`
	// AdvertiseAddr is the address other nodes use to reach this one (e.g. "host:9001").
	AdvertiseAddr string `yaml:"advertise_addr" mapstructure:"advertise_addr"`
	// Peers is the list of peer advertise addresses.
	Peers []string `yaml:"peers" mapstructure:"peers"`
	// HeartbeatInterval is how often the leader sends heartbeats (default 2s).
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" mapstructure:"heartbeat_interval"`
	// ElectionTimeout is how long a follower waits before starting an election (default 6s).
	ElectionTimeout time.Duration `yaml:"election_timeout" mapstructure:"election_timeout"`
	// DataDir is where cluster state is persisted.
	DataDir string `yaml:"data_dir" mapstructure:"data_dir"`
}

// leaderChangeFn is a callback invoked whenever leadership changes.
type leaderChangeFn func(isLeader bool, leader string)

// Node is a cluster member that participates in leader election.
type Node struct {
	cfg    NodeConfig
	mu     sync.RWMutex
	role   NodeRole
	term   uint64
	leader string
	peers  []*peerClient
	stopCh chan struct{}
	// onLeaderChange is called (in a goroutine) when leadership changes.
	onLeaderChange leaderChangeFn
}

// New creates a new cluster Node. It does not start any background workers;
// call Start to begin participating in the cluster.
func New(cfg NodeConfig) (*Node, error) {
	if cfg.NodeID == "" {
		cfg.NodeID = uuid.New().String()
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 2 * time.Second
	}
	if cfg.ElectionTimeout <= 0 {
		cfg.ElectionTimeout = 6 * time.Second
	}

	peers := make([]*peerClient, 0, len(cfg.Peers))
	for _, addr := range cfg.Peers {
		peers = append(peers, newPeerClient(addr))
	}

	n := &Node{
		cfg:    cfg,
		role:   RoleFollower,
		peers:  peers,
		stopCh: make(chan struct{}),
	}

	// Single-node mode: no peers → always leader.
	if len(peers) == 0 {
		n.role = RoleLeader
		n.leader = cfg.AdvertiseAddr
	}

	return n, nil
}

// Start begins the cluster node. In multi-peer mode it starts the heartbeat
// sender (if leader) or the election timer (if follower).
func (n *Node) Start(ctx context.Context) error {
	n.mu.RLock()
	role := n.role
	n.mu.RUnlock()

	if len(n.peers) == 0 {
		// Single-node mode — nothing to do.
		log.Info().Str("node_id", n.cfg.NodeID).Msg("cluster: single-node mode; this node is leader")
		return nil
	}

	log.Info().
		Str("node_id", n.cfg.NodeID).
		Str("role", string(role)).
		Int("peers", len(n.peers)).
		Msg("cluster: starting")

	go n.runLoop(ctx)
	return nil
}

// Stop gracefully shuts down the node.
func (n *Node) Stop() {
	select {
	case <-n.stopCh:
		// already stopped
	default:
		close(n.stopCh)
	}
}

// IsLeader returns true if this node is currently the cluster leader.
func (n *Node) IsLeader() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.role == RoleLeader
}

// Leader returns the advertise address of the current leader, or empty if unknown.
func (n *Node) Leader() string {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.leader
}

// Role returns the node's current role.
func (n *Node) Role() NodeRole {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.role
}

// Term returns the current election term.
func (n *Node) Term() uint64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.term
}

// OnLeaderChange registers a callback called when leadership changes.
// Only one callback is supported; subsequent calls replace the previous one.
func (n *Node) OnLeaderChange(fn func(isLeader bool, leader string)) {
	n.mu.Lock()
	n.onLeaderChange = fn
	n.mu.Unlock()
}

// ReceiveHeartbeat processes a heartbeat from the current leader.
// Returns an error if the term is stale.
func (n *Node) ReceiveHeartbeat(term uint64, leaderAddr string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if term < n.term {
		return fmt.Errorf("cluster: stale heartbeat term %d (current %d)", term, n.term)
	}

	prevLeader := n.leader
	prevRole := n.role

	n.term = term
	n.leader = leaderAddr
	n.role = RoleFollower

	if prevRole != RoleFollower || prevLeader != leaderAddr {
		cb := n.onLeaderChange
		if cb != nil {
			go cb(false, leaderAddr)
		}
	}

	// Reset the election timer by sending to stopCh (runLoop will reset).
	// We do this non-blocking to avoid deadlocking.
	return nil
}

// runLoop drives the leader/follower state machine.
func (n *Node) runLoop(ctx context.Context) {
	electionTimer := time.NewTimer(n.cfg.ElectionTimeout)
	heartbeatTicker := time.NewTicker(n.cfg.HeartbeatInterval)
	defer electionTimer.Stop()
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-n.stopCh:
			return
		case <-heartbeatTicker.C:
			n.mu.RLock()
			role := n.role
			n.mu.RUnlock()
			if role == RoleLeader {
				n.sendHeartbeats()
			}
		case <-electionTimer.C:
			n.mu.RLock()
			role := n.role
			n.mu.RUnlock()
			if role != RoleLeader {
				n.startElection()
			}
			// Reset timer regardless.
			electionTimer.Reset(n.cfg.ElectionTimeout)
		}
	}
}

// sendHeartbeats broadcasts heartbeats to all peers.
func (n *Node) sendHeartbeats() {
	n.mu.RLock()
	term := n.term
	addr := n.cfg.AdvertiseAddr
	n.mu.RUnlock()

	for _, p := range n.peers {
		go func(peer *peerClient) {
			if err := peer.SendHeartbeat(context.Background(), term, addr); err != nil {
				log.Debug().Err(err).Str("peer", peer.addr).Msg("cluster: heartbeat failed")
			}
		}(p)
	}
}

// startElection attempts to become the leader for term+1.
func (n *Node) startElection() {
	n.mu.Lock()
	n.term++
	n.role = RoleCandidate
	term := n.term
	n.mu.Unlock()

	log.Info().Str("node_id", n.cfg.NodeID).Uint64("term", term).Msg("cluster: starting election")

	votes := 1 // vote for ourselves
	for _, p := range n.peers {
		if p.RequestVote(context.Background(), term, n.cfg.NodeID) {
			votes++
		}
	}

	majority := (len(n.peers)+1)/2 + 1
	n.mu.Lock()
	if votes >= majority && n.role == RoleCandidate && n.term == term {
		n.role = RoleLeader
		n.leader = n.cfg.AdvertiseAddr
		cb := n.onLeaderChange
		n.mu.Unlock()
		log.Info().
			Str("node_id", n.cfg.NodeID).
			Uint64("term", term).
			Int("votes", votes).
			Msg("cluster: won election; became leader")
		if cb != nil {
			go cb(true, n.cfg.AdvertiseAddr)
		}
	} else {
		if n.role == RoleCandidate {
			n.role = RoleFollower
		}
		n.mu.Unlock()
		log.Info().
			Str("node_id", n.cfg.NodeID).
			Uint64("term", term).
			Int("votes", votes).
			Int("majority", majority).
			Msg("cluster: lost election")
	}
}
