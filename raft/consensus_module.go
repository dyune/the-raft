package raft

import (
	"sync"
	"time"
)

type Server struct {
	// TODO: None for now
}

type State int

const (
	Follower = iota
	Candidate
	Leader
	Unhealthy
)

func (s State) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	case Unhealthy:
		return "Unhealthy"
	default:
		panic("Impossible type for State in ConsensusModule struct")
	}
}

type ConsensusModule struct {
	mu          sync.Mutex
	id          int
	peerIds     []int
	server      *Server
	state       State
	currentTerm int
	resetEvent  time.Time
	votedFor    int
}
