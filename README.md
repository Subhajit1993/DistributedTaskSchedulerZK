# Distributed Task Scheduler with ZooKeeper

A robust distributed task scheduler built using Go and Apache ZooKeeper, designed for high availability and fault tolerance. This system enables distributed task processing across multiple workers with automatic leader election and failure recovery.

## Features

- Distributed task scheduling and processing
- Automatic leader election
- Worker health monitoring
- Fault tolerance and automatic recovery
- Dynamic worker scaling
- Task redistribution on worker failure
- Real-time system state management

## Prerequisites

- Go 1.16 or higher
- Apache ZooKeeper 3.7 or higher
- `github.com/go-zookeeper/zk` package

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/distributed-task-scheduler.git
cd distributed-task-scheduler
```

Install dependencies:

```bash
go mod download
```

Configuration
The system uses several configurable constants in node.go:
```go
const (
    zkRoot      = "/distributed-worker"
    leaderPath  = "/leader"
    workersPath = "/workers"
    tasksPath   = "/tasks"
    zkTimeout   = 10 * time.Second
    sessionTimeout = 30 * time.Second
)
```

## Usage
```bash
go run main.go
```