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

type AppendEntryReply struct {
	replyTerm int
	success   bool
}

const ResetTime = time.Duration(300) * time.Millisecond
const DebugCm = 1

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
		if elapsed := time.Since(cm.resetEvent); elapsed > ResetTime {
			cm.startElection()
			cm.mu.Unlock()
			return
		}
		cm.mu.Unlock()
	}
}

func (cm *ConsensusModule) dlog(format string, args ...any) {
	if DebugCm > 0 {
		format = fmt.Sprintf("[%d] ", cm.id) + format
		log.Printf(format, args...)
	}
}

func (server *Server) call(id int, method string, args any, reply any) error {
	return errors.New("not implemented yet")
}

func (cm *ConsensusModule) becomeFollower(term int) {
	cm.dlog("demoted to follower in term: %d", term)
	cm.currentTerm = term
	cm.votedFor = -1
	cm.resetEvent = time.Now()
	go cm.runElectionTimer()
}

func (cm *ConsensusModule) sendHeartBeats() {
	cm.mu.Lock()
	term := cm.currentTerm
	id := cm.id
	cm.mu.Unlock()

	for _, peerId := range cm.peerIds {
		args := AppendEntryArgs{term, id}
		go func() {
			cm.dlog("in term %d sent heartbeat to a peer of id %d", term, id)
			var reply AppendEntryReply
			cm.server.call(peerId, "ConsensusModule.AppendEntry", args, &reply)
			if term <= reply.replyTerm {
				cm.dlog("out of term while sending heartbeats")
				cm.becomeFollower(reply.replyTerm)
			}
		}()
	}
}

func (cm *ConsensusModule) becomeLeader() {
	cm.state = Leader
	cm.dlog("has become the leader on term %d", cm.currentTerm)
	go func() {
		ticker := time.NewTimer(40 * time.Millisecond)
		defer ticker.Stop()
		for {
			<-ticker.C
			cm.sendHeartBeats()
			cm.mu.Lock()
			if cm.state != Leader {
				cm.dlog("was voted out")
				cm.mu.Unlock()
				return
			}
			cm.mu.Unlock()
		}
	}()
}

type RequestVoteArgs struct {
	term        int
	candidateId int
}

type RequestVoteReply struct {
	term        int
	voteGranted bool
}

func (cm *ConsensusModule) requestVote(args RequestVoteArgs, reply *RequestVoteReply) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.state == Unhealthy {
		return nil
	}

	if args.term > cm.currentTerm {
		cm.dlog("out of term")
		cm.becomeFollower(args.term)
	}
	if args.term == cm.currentTerm && (cm.votedFor == -1 || cm.votedFor == args.candidateId) {
		reply.voteGranted = true
		cm.votedFor = args.candidateId
		cm.resetEvent = time.Now()
	} else {
		reply.voteGranted = false
	}
	reply.term = cm.currentTerm
	cm.dlog("replied RequestVote: %+v", reply)
	return nil
}

func (cm *ConsensusModule) appendEntries(args AppendEntryArgs, reply *AppendEntryReply) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.state == Unhealthy {
		return nil
	}
	if args.term > cm.currentTerm {
		cm.dlog("out of term")
		cm.becomeFollower(args.term)
	}
	reply.success = false
	if args.term == cm.currentTerm {
		if cm.state == Leader {
			cm.dlog("this cm with id=%d was voted out", cm.id)
			cm.becomeFollower(args.term)
		}
		cm.resetEvent = time.Now()
		reply.success = true
	}
	reply.replyTerm = cm.currentTerm
	cm.dlog("replied AppendEntry: %+v", reply)
	return nil
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
					cm.becomeFollower(result.term)
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
