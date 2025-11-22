# Raft Implementation Design

## Overview

This document describes the Raft consensus algorithm implementation in the distributed key-value store.

## Raft States

A node can be in one of three states:

1. **Follower**: Passive state, receives log entries from leader
2. **Candidate**: Temporary state during leader election
3. **Leader**: Active state, handles client requests and replicates log entries

## State Transitions

```
Follower → Candidate (election timeout)
Candidate → Leader (majority votes)
Candidate → Follower (higher term discovered)
Leader → Follower (higher term discovered)
```

## Persistent State (on stable storage)

- `currentTerm`: Latest term seen
- `votedFor`: Candidate ID that received vote in current term
- `log[]`: Log entries (index 0 is dummy entry)

## Volatile State

- `commitIndex`: Highest log entry known to be committed
- `lastApplied`: Highest log entry applied to state machine

## Leader Volatile State

- `nextIndex[]`: For each server, index of next log entry to send
- `matchIndex[]`: For each server, index of highest log entry known to be replicated

## RPCs

### AppendEntries

**Purpose**: Replicate log entries and serve as heartbeat

**Request:**
- `term`: Leader's term
- `leaderId`: Leader's ID
- `prevLogIndex`: Index of log entry immediately preceding new ones
- `prevLogTerm`: Term of `prevLogIndex` entry
- `entries[]`: Log entries to store (empty for heartbeat)
- `leaderCommit`: Leader's `commitIndex`

**Response:**
- `term`: Current term (for leader to update itself)
- `success`: True if follower contained entry matching `prevLogIndex` and `prevLogTerm`

**Receiver Implementation:**
1. Reply false if `term < currentTerm`
2. Reply false if log doesn't contain entry at `prevLogIndex` with term `prevLogTerm`
3. If existing entry conflicts with new one (same index, different term), delete entry and all that follow
4. Append any new entries not already in log
5. If `leaderCommit > commitIndex`, set `commitIndex = min(leaderCommit, index of last new entry)`

### RequestVote

**Purpose**: Request votes during election

**Request:**
- `term`: Candidate's term
- `candidateId`: Candidate requesting vote
- `lastLogIndex`: Index of candidate's last log entry
- `lastLogTerm`: Term of candidate's last log entry

**Response:**
- `term`: Current term (for candidate to update itself)
- `voteGranted`: True means candidate received vote

**Receiver Implementation:**
1. Reply false if `term < currentTerm`
2. If `votedFor` is null or `candidateId`, and candidate's log is at least as up-to-date as receiver's log, grant vote

## Election Process

1. **Election Timeout**: Random between 150ms-300ms
2. **Convert to Candidate**: Increment `currentTerm`, vote for self, reset election timer
3. **Request Votes**: Send `RequestVote` RPCs to all other servers
4. **Become Leader**: If votes received from majority, become leader
5. **Send Heartbeats**: Upon election, send initial empty `AppendEntries` RPCs

## Log Replication

1. **Client Request**: Leader receives client request
2. **Append to Log**: Leader appends entry to local log
3. **Replicate**: Leader sends `AppendEntries` RPCs to all followers
4. **Majority Commit**: When majority of followers have replicated entry, leader commits
5. **Apply**: Leader applies entry to state machine and responds to client
6. **Notify Followers**: Leader includes `leaderCommit` in subsequent `AppendEntries` RPCs

## Safety Properties

- **Election Safety**: At most one leader can be elected in a given term
- **Leader Append-Only**: Leader never overwrites or deletes entries in its log
- **Log Matching**: If two logs contain an entry with the same index and term, then the logs are identical in all preceding entries
- **Leader Completeness**: If a log entry is committed in a given term, then that entry will be present in the logs of all leaders of higher-numbered terms

## Implementation Notes

### MVP v0 (Current)
- Single-node mode (Raft disabled)
- Direct state machine application

### MVP v1 (Next)
- Full Raft implementation
- Leader election
- Log replication
- Client redirects (followers redirect to leader)

### MVP v2+
- Persistence (WAL + snapshots)
- Dynamic membership
- Configuration changes


