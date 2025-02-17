package workers

import (
	"encoding/json"
	"fmt"
	"github.com/go-zookeeper/zk"
	"log"
	"path"
	"sort"
	"time"
)

func (wm *WorkerManager) ElectLeader() error {
	leaderNodePath := path.Join(zkRoot, leaderPath, "leader_")
	data, err := json.Marshal(wm.worker)
	if err != nil {
		fmt.Printf("failed to marshal worker data: %v\n", err)
		return err
	}
	createdNode := ""
	if wm.currentLeaderNode == "" {
		// Create a new ephemeral sequential node
		createdNode, err = wm.Conn.Create(leaderNodePath, data, zk.FlagEphemeral|zk.FlagSequence, zk.WorldACL(zk.PermAll))
		if err != nil {
			fmt.Printf("failed to create leader node: %v\n", err)
			return err
		}

		fmt.Printf("created leader node: %s\n", createdNode)

		// Update currentNode to reflect the leader election node (e.g., leader_00001)
		wm.mu.Lock()
		wm.currentLeaderNode = createdNode
		wm.mu.Unlock()
	} else {
		// It is possible that the current leader node is already set and this node was a follower before. Now due to some reason, it is trying to re-elect the leader.
		createdNode = wm.currentLeaderNode
	}

	// Parse the created node's sequence to identify leadership
	children, _, err := wm.Conn.Children(path.Join(zkRoot, leaderPath))
	if err != nil {
		return fmt.Errorf("failed to list leader nodes: %v", err)
	}

	// Sort nodes to find the one with the smallest sequence
	sort.Strings(children)
	if len(children) > 0 && path.Base(createdNode) == children[0] {
		// This worker is the leader
		wm.becomeLeader()
		fmt.Printf("worker %s is leader\n", wm.worker.ID)
	} else {
		fmt.Printf("worker %s is not leader\n", wm.worker.ID)
		go func() {
			err := wm.watchLeader()
			if err != nil {
				fmt.Printf("failed to watch leader: %v\n", err)
			}
		}()
	}
	return nil
}

func (wm *WorkerManager) becomeLeader() {
	wm.mu.Lock()
	wm.isLeader = true
	wm.mu.Unlock()

	log.Printf("Node %s became leader", wm.worker.ID)

	// Start leader duties
	go wm.performLeaderDuties()
}

func (wm *WorkerManager) watchLeader() error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		// Get our position and all candidates
		position, children, err := wm.getNodePosition()
		if err != nil {
			return err
		}
		if position == 0 {
			// We're the leader
			wm.becomeLeader()
			fmt.Printf("worker %s is leader\n", wm.worker.ID)
		} else if position > 0 {
			// Watch the node ahead of us
			leaderNode := path.Join(zkRoot, leaderPath, children[position-1])
			exists, _, nodeEvents, err := wm.Conn.ExistsW(leaderNode)
			if err != nil {
				return err
			}
			if !exists {
				// The node ahead of us disappeared
				wm.ElectLeader()
			} else {
				// Wait for the node to change
				event := <-nodeEvents
				if event.Type == zk.EventNodeDeleted {
					wm.ElectLeader()
				}
			}

		}
	}
	return nil
}
