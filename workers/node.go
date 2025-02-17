package workers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path"
	"sort"
	"sync"
	"time"
)

import (
	"github.com/go-zookeeper/zk"
)

const placeholder = "-7"

const (
	// ZooKeeper paths
	zkRoot      = "/distributed-worker" + placeholder
	leaderPath  = "/leader" + placeholder
	workersPath = "/workers" + placeholder
	tasksPath   = "/tasks" + placeholder

	// Connection settings
	zkTimeout      = 10 * time.Second
	sessionTimeout = 30 * time.Second
)

// Worker Purpose: Represents a single worker node in the system
type Worker struct {
	ID    string
	Host  string
	Port  int
	Tasks int
}

type Task struct {
	ID          string
	Description string
	Status      string
	AssignedTo  string
	CreatedAt   time.Time
}

// WorkerManager Purpose: Manages worker operations and coordinates with ZooKeeper
type WorkerManager struct {
	Conn              *zk.Conn
	worker            Worker
	isLeader          bool
	mu                sync.RWMutex
	tasks             map[string]Task
	currentLeaderNode string // Store our node's full path
	currentWorkerNode string // Store our node's full path
	TaskRunnerChanged chan bool
}

func (wm *WorkerManager) getWorkers() ([]Worker, error) {
	// Get all children in workers path
	children, _, err := wm.Conn.Children(path.Join(zkRoot, workersPath))
	if err != nil {
		return nil, err
	}

	// Get worker data for each child
	workers := make([]Worker, 0, len(children))
	for _, child := range children {
		workerPath := path.Join(zkRoot, workersPath, child)
		data, _, err := wm.Conn.Get(workerPath)
		if err != nil {
			return nil, err
		}

		// Unmarshal data into Worker struct
		var worker Worker
		if err := json.Unmarshal(data, &worker); err != nil {
			return nil, err
		}
		workers = append(workers, worker)

	}
	return workers, nil
}

func (wm *WorkerManager) CheckNodeStatus() error {
	// Check if the current node has been created
	exists, _, err := wm.Conn.Exists(wm.currentWorkerNode)
	if err != nil {
		return fmt.Errorf("error checking existence of current node: %v", err)
	}

	if !exists {
		// Node hasn't registered itself
		return fmt.Errorf("current node (%s) is not registered", wm.currentWorkerNode)
	}

	// Check if there are any other nodes in the leaderPath
	children, _, err := wm.Conn.Children(path.Join(zkRoot, leaderPath))
	if err != nil {
		return fmt.Errorf("error fetching leader path children: %v", err)
	}

	if len(children) == 0 {
		// No nodes in leaderPath: first node attempting election
		return nil
	}

	return nil
}

func (wm *WorkerManager) getNodePosition() (int, []string, error) {
	// Get all children in leader path
	children, _, err := wm.Conn.Children(path.Join(zkRoot, leaderPath))
	if err != nil {
		return -1, nil, err
	}

	// If no children exist, return early
	if len(children) == 0 {
		return -1, children, fmt.Errorf("no nodes found in leader path")
	}

	// Sort children to maintain consistent order
	sort.Strings(children)

	// Find our node's name (without full path)
	currentLeaderNodeName := path.Base(wm.currentLeaderNode)

	// Find our position
	for i, child := range children {
		if child == currentLeaderNodeName {
			return i, children, nil
		}
	}

	return -1, children, fmt.Errorf("current node not found")
}

func (wm *WorkerManager) performLeaderDuties() {
	// Run periodic checks and task distribution
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		// Check if still leader
		wm.mu.RLock()
		if !wm.isLeader {
			wm.mu.RUnlock()
			return
		}
		wm.mu.RUnlock()

		// Leader's main tasks:

		// 1. Get active workers
		workers, err := wm.getWorkers()
		if err != nil {
			log.Printf("Failed to get workers: %v", err)
			continue
		}

		// 2. Get pending tasks
		tasks, err := wm.getUnassignedTasks()
		if err != nil {
			log.Printf("Failed to get unassigned tasks: %v", err)
			continue
		}

		for _, task := range tasks {
			if task.Status == "pending" {
				// Assign task to a worker
				wm.assignTasks(tasks, workers)
			}
		}
		// 4. Monitor worker health
		wm.checkWorkerHealth()
	}
}
func NewWorkerManager(hosts []string, worker Worker) (*WorkerManager, error) {
	conn, _, err := zk.Connect(hosts, sessionTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ZooKeeper: %v", err)
	}
	wm := &WorkerManager{
		Conn:              conn,
		worker:            worker,
		tasks:             make(map[string]Task),
		TaskRunnerChanged: make(chan bool, 1),
	}

	// Ensure base paths exist
	paths := []string{zkRoot, path.Join(zkRoot, leaderPath),
		path.Join(zkRoot, workersPath), path.Join(zkRoot, tasksPath)}

	for _, p := range paths {
		exists, _, err := conn.Exists(p)
		if err != nil {
			return nil, err
		}
		if !exists {
			_, err = conn.Create(p, []byte{}, 0, zk.WorldACL(zk.PermAll))
			if err != nil && !errors.Is(err, zk.ErrNodeExists) {
				return nil, err
			}
		}
	}
	return wm, nil
}

func (wm *WorkerManager) Register() error {
	data, err := json.Marshal(wm.worker)
	if err != nil {
		fmt.Printf("failed to marshal worker data: %v\n", err)
		return err
	}
	workerPath := path.Join(zkRoot, workersPath, wm.worker.ID)
	_, err = wm.Conn.Create(workerPath, data, zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
	if err != nil {
		fmt.Printf("failed to register worker: %v\n", err)
		return err
	}

	// Set currentNode to the created node's path
	wm.mu.Lock()
	wm.currentWorkerNode = workerPath
	wm.mu.Unlock()

	fmt.Printf("registered worker %s\n", wm.worker.ID)
	return nil
}

// watchLeader Purpose: Watches the leader node for changes
