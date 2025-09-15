package raft

import (
	"time"
)

const ELECTION_TIME = time.Duration(300) * time.Millisecond

func runElectionTimer(cm *ConsensusModule) {
	ticker := time.NewTimer(10 * time.Millisecond)
	startTerm := cm.currentTerm
	for {
		<-ticker.C // sleep for 10 ms
		cm.mu.Lock()
		if cm.state != Candidate && cm.state != Follower { // you are a Leader, no timeout needed
			cm.mu.Unlock()
			return
		}
		if cm.currentTerm != startTerm { // new elections are starting
			cm.mu.Unlock()
			return
		}
		if elapsed := time.Since(cm.resetEvent); elapsed > ELECTION_TIME {
			cm.startElection()
			cm.mu.Unlock()
			return
		}
		cm.mu.Unlock()
	}
}
