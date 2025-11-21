# Architecture Overview

## System Design

Conflux is a distributed key-value store built on the Raft consensus algorithm. The system is designed for strong consistency, high availability, and operational simplicity.

## High-Level Architecture

```mermaid
graph TB
    subgraph "Client Layer"
        C1[Client 1]
        C2[Client 2]
        C3[Client 3]
    end
    
    subgraph "Raft Cluster"
        N1[Node 1<br/>Leader]
        N2[Node 2<br/>Follower]
        N3[Node 3<br/>Follower]
    end
    
    subgraph "Storage Layer"
        W1[WAL + Snapshots]
        W2[WAL + Snapshots]
        W3[WAL + Snapshots]
    end
    
    subgraph "Observability"
        P[Prometheus]
        G[Grafana]
    end
    
    C1 & C2 & C3 -->|HTTP/gRPC| N1 & N2 & N3
    N1 <-->|Raft RPC| N2
    N2 <-->|Raft RPC| N3
    N3 <-->|Raft RPC| N1
    N1 --> W1
    N2 --> W2
    N3 --> W3
    N1 & N2 & N3 -->|Metrics| P
    P --> G
```

## Core Components

### 1. Raft Consensus Layer

**Purpose**: Ensures all nodes agree on the order of operations

**Key Features**:
- Leader election with randomized timeouts (150-300ms)
- Log replication with consistency checks
- Automatic failure detection and recovery
- Split-brain prevention via majority quorum

**Implementation**: [`pkg/raft/`](../../pkg/raft/)

**State Machine**:
```mermaid
stateDiagram-v2
    [*] --> Follower
    Follower --> Candidate: Election timeout
    Candidate --> Leader: Receives majority votes
    Candidate --> Follower: Discovers leader or new term
    Leader --> Follower: Discovers higher term
    Follower --> Follower: Receives valid AppendEntries
```

### 2. Key-Value State Machine

**Purpose**: Applies committed Raft log entries to the in-memory store

**Operations**:
- `PUT(key, value)` - Set or update a key
- `GET(key)` - Retrieve a value
- `DELETE(key)` - Remove a key

**Implementation**: [`pkg/kv/`](../../pkg/kv/)

**Characteristics**:
- Deterministic: Same log → same state
- In-memory for fast access
- Thread-safe with mutex protection
- Snapshot support for compaction

### 3. Persistence Layer

**Components**:
- **Write-Ahead Log (WAL)**: Durable log of all operations
- **Snapshots**: Periodic state checkpoints for faster recovery

**Implementation**: [`pkg/wal/`](../../pkg/wal/), [`pkg/snapshot/`](../../pkg/snapshot/)

**Recovery Process**:
1. Load latest snapshot (if exists)
2. Replay WAL entries since snapshot
3. Rebuild in-memory state

### 4. HTTP API Layer

**Purpose**: Provides RESTful interface for clients

**Endpoints**:
- `GET /kv/{key}` - Read value
- `PUT /kv/{key}` - Write value
- `DELETE /kv/{key}` - Delete value
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics

**Implementation**: [`pkg/api/`](../../pkg/api/)

**Features**:
- Leader redirection for writes
- Panic recovery middleware
- Structured logging
- Prometheus instrumentation

### 5. Kubernetes Operator

**Purpose**: Automates cluster lifecycle management

**Responsibilities**:
- Create and manage StatefulSets
- Configure DNS-based peer discovery
- Manage PersistentVolumeClaims
- Update cluster configuration
- Monitor cluster health

**Implementation**: [`operator/`](../../operator/)

**CRD**: `RaftCluster`
```yaml
apiVersion: raft.conflux.io/v1alpha1
kind: RaftCluster
metadata:
  name: raft-sample
spec:
  replicas: 3
  image: hugo/raft-node:latest
  resources:
    requests:
      memory: "256Mi"
      cpu: "100m"
```

## Data Flow

### Write Operation

```mermaid
sequenceDiagram
    participant C as Client
    participant L as Leader
    participant F1 as Follower 1
    participant F2 as Follower 2
    
    C->>L: PUT /kv/key {"value":"data"}
    L->>L: Append to local log
    L->>L: Persist to WAL
    par Replicate to Followers
        L->>F1: AppendEntries RPC
        L->>F2: AppendEntries RPC
    end
    F1->>F1: Append to log + WAL
    F2->>F2: Append to log + WAL
    F1-->>L: Success
    F2-->>L: Success
    L->>L: Commit entry (majority achieved)
    L->>L: Apply to state machine
    L-->>C: 200 OK
    par Notify Followers
        L->>F1: AppendEntries (with new commitIndex)
        L->>F2: AppendEntries (with new commitIndex)
    end
    F1->>F1: Apply to state machine
    F2->>F2: Apply to state machine
```

### Read Operation

**From Leader** (Linearizable):
```
Client → Leader → State Machine → Response
```

**From Follower** (Potentially Stale):
```
Client → Follower → State Machine → Response
```

### Leader Election

```mermaid
sequenceDiagram
    participant F as Follower
    participant C as Candidate
    participant F1 as Follower 1
    participant F2 as Follower 2
    
    Note over F: Election timeout expires
    F->>C: Transition to Candidate
    C->>C: Increment term, vote for self
    par Request Votes
        C->>F1: RequestVote RPC
        C->>F2: RequestVote RPC
    end
    F1-->>C: VoteGranted: true
    F2-->>C: VoteGranted: true
    Note over C: Received majority (3/3)
    C->>C: Transition to Leader
    par Send Heartbeats
        C->>F1: AppendEntries (heartbeat)
        C->>F2: AppendEntries (heartbeat)
    end
```

## Consistency Model

### Guarantees

- **Linearizability**: All operations appear to execute atomically at a single point in time
- **Durability**: Committed writes survive node failures (WAL + majority replication)
- **Total Ordering**: All nodes apply operations in the same order

### Trade-offs

- **Availability**: Requires majority quorum (2/3 nodes for 3-node cluster)
- **Latency**: Write latency includes network round-trip for replication
- **Read Consistency**: Reads from followers may be stale

## Failure Scenarios

### Leader Failure

1. Followers detect missing heartbeats (election timeout)
2. Followers transition to candidates
3. New leader elected via majority vote
4. New leader replicates uncommitted entries
5. Cluster resumes normal operation

**Recovery Time**: ~150-300ms (election timeout)

### Follower Failure

1. Leader detects failed AppendEntries RPCs
2. Leader continues with remaining followers
3. Failed follower restarts and rejoins
4. Leader brings follower up-to-date via log replication

**Impact**: None (if majority still available)

### Network Partition

**Scenario**: 3-node cluster splits into [1] and [2] partitions

- **Majority partition [2]**: Continues operating normally
- **Minority partition [1]**: Cannot elect leader, rejects writes
- **After partition heals**: Minority rejoins, syncs from leader

## Performance Characteristics

### Throughput

- **Writes**: Limited by leader's disk I/O and network to followers
- **Reads**: Scales linearly with number of nodes (if stale reads acceptable)

### Latency

- **Write latency**: 1-10ms (local network) to 50-100ms (cross-region)
  - Components: Leader log append + Network RTT + Follower append
- **Read latency**: <1ms (in-memory lookup)

### Scalability

- **Cluster size**: Typically 3-7 nodes (odd numbers for quorum)
- **Data size**: Limited by available memory (snapshots help)
- **Request rate**: 10k-100k ops/sec depending on hardware

## Design Decisions

### Why Raft?

- **Understandability**: Easier to reason about than Paxos
- **Proven**: Used in production by etcd, Consul, CockroachDB
- **Strong consistency**: Linearizable reads/writes
- **Leader-based**: Simplifies client interaction

### Why In-Memory State Machine?

- **Performance**: Sub-millisecond read latency
- **Simplicity**: No external database dependencies
- **Snapshots**: Periodic checkpoints prevent unbounded memory growth

### Why Kubernetes Operator?

- **Automation**: Declarative cluster management
- **Cloud-native**: Integrates with Kubernetes ecosystem
- **Scalability**: Easy horizontal scaling via StatefulSets

## See Also

- [Raft Consensus Details](raft-consensus.md)
- [Persistence Design](persistence.md)
- [Operator Design](operator.md)
- [Raft Paper](https://raft.github.io/raft.pdf)
