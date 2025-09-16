package raft

import (
	"errors"
	"fmt"
	"log"
	"time"
)

type VoteRequest struct {
	currTerm    int
	candidateId int
}

type Reply struct {
	term     int
	votedFor bool
}

const ELECTION_TIME = time.Duration(300) * time.Millisecond
const DEBUG_CM = 1

func (cm *ConsensusModule) runElectionTimer() {
	ticker := time.NewTimer(10 * time.Millisecond)
	startTerm := cm.currentTerm
	for {
		<-ticker.C // sleep for 10 ms
		cm.mu.Lock()
		if cm.state != Candidate && cm.state != Follower { // you are a Leader, cancel election timer
			cm.mu.Unlock()
			return
		}
		if cm.currentTerm != startTerm { // new elections are starting, end here
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

func (cm *ConsensusModule) dlog(format string, args ...any) {
	if DEBUG_CM > 0 {
		format = fmt.Sprintf("[%d] ", cm.id) + format
		log.Printf(format, args...)
	}
}

func (server *Server) call(id int, method string, request VoteRequest, result *Reply) error {
	return errors.New("not implemented yet")
}

func (cm *ConsensusModule) becomeFollower() {
	return
}

func (cm *ConsensusModule) becomeLeader() {
	return
}

func (cm *ConsensusModule) startElection() {
	/*
		Switch the state to candidate and increment the term, because that's what the algorithm dictates for every election.
		Send RV RPCs to all peers, asking them to vote for us in this election.
		Wait for replies to these RPCs and count if we got enough votes to become a leader.
	*/
	cm.state = Candidate
	cm.currentTerm += 1
	cm.resetEvent = time.Now()
	cm.votedFor = cm.id
	votes := 1
	savedTerm := cm.currentTerm
	for _, peerId := range cm.peerIds {
		go func(id int) {
			var result Reply
			if err := cm.server.call(
				id, "ConsensusModule.RequestVote", VoteRequest{cm.currentTerm, cm.id}, &result); err != nil {
				cm.mu.Unlock()
				if cm.state != Candidate {
					cm.dlog("was forced to step down from election. "+
						"While waiting for reply, state became: %v", cm.state)
					return
				}
				if result.term > savedTerm {
					cm.dlog("has an outdated term."+
						"While waiting for reply, higher term %d was found.", result.term)
					cm.becomeFollower()
					return
				} else if result.term == savedTerm {
					votes++
					if votes > (len(cm.peerIds)/2)+1 {
						cm.dlog("has attained majority votes. Becoming leader...")
						cm.becomeLeader()
						return
					}
				}
			}
		}(peerId)
	}
	/*
		After the votes are cast, keep going. The launched go routines in the for-loop will handle the vote results.
		If you become a leader, the periodic nature of `runElectionTimer` will find out you are a leader and stop.
		If this election fails, you will keep on running the election timer as usual. Like this, we don't ever block.
	*/
	go cm.runElectionTimer()
}
