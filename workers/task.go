package workers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-zookeeper/zk"
	"log"
	"path"
	"time"
)

func (wm *WorkerManager) AddTask(task Task) error {
	// Marshal the task data to JSON format
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task data: %v", err)
	}

	// Create a unique path for the task in ZooKeeper
	taskPath := path.Join(zkRoot, tasksPath, task.ID) // Use task.ID to uniquely identify the task

	// Create the task node in ZooKeeper
	_, err = wm.Conn.Create(taskPath, data, zk.FlagEphemeral, zk.WorldACL(zk.PermAll))
	if err != nil {
		if errors.Is(err, zk.ErrNodeExists) {
			return fmt.Errorf("task %s already exists in ZooKeeper", task.ID)
		}
		return fmt.Errorf("failed to create task in ZooKeeper: %v", err)
	}

	log.Printf("Task added successfully: %s", task.ID)
	return nil
}

func (wm *WorkerManager) getUnassignedTasks() ([]Task, error) {
	// Get all children in tasks path
	children, _, err := wm.Conn.Children(path.Join(zkRoot, tasksPath))
	if err != nil {
		return nil, err
	}

	// Get task data for each child
	tasks := make([]Task, 0, len(children))
	for _, child := range children {
		taskPath := path.Join(zkRoot, tasksPath, child)
		data, _, err := wm.Conn.Get(taskPath)
		if err != nil {
			return nil, err
		}

		// Unmarshal data into Task struct
		var task Task
		if err := json.Unmarshal(data, &task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (wm *WorkerManager) assignTasks(tasks []Task, workers []Worker) {
	// Assign tasks to workers
	if len(workers) == 0 {
		return
	}

	// Simple round-robin task assignment
	for i, task := range tasks {
		workerIndex := i % len(workers)
		worker := workers[workerIndex]

		// Assign task to worker
		task.AssignedTo = worker.ID
		task.Status = "assigned"

		// Update task in ZooKeeper
		if err := wm.updateTask(task); err != nil {
			log.Printf("Failed to assign task %s: %v", task.ID, err)
			continue
		}

		wm.TaskRunnerChanged <- true
	}
}

func (wm *WorkerManager) getAssignedTasks() []Task {
	// Get all children in tasks path
	children, _, err := wm.Conn.Children(path.Join(zkRoot, tasksPath))
	if err != nil {
		log.Printf("Failed to get tasks: %v", err)
		return nil
	}
	currWorker := wm.worker.ID

	// Get task data for current worker
	tasks := make([]Task, 0)
	for _, child := range children {
		taskPath := path.Join(zkRoot, tasksPath, child)
		data, _, err := wm.Conn.Get(taskPath)
		if err != nil {
			log.Printf("Failed to get task data: %v", err)
			continue
		}

		// Unmarshal data into Task struct
		var task Task
		if err := json.Unmarshal(data, &task); err != nil {
			log.Printf("Failed to unmarshal task data: %v", err)
			continue
		}

		// Check if the task is assigned to the current worker
		if task.AssignedTo == currWorker {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

func (wm *WorkerManager) processAssignedTasks() {
	// Get all tasks assigned to the worker
	assignedTasks := wm.getAssignedTasks()
	// Simulate worker processing tasks
	for _, task := range assignedTasks {
		log.Printf("Worker %s is processing task %s", wm.worker.ID, task.ID)
		// Simulate task completion
		// Goroutine to simulate task completion

		go func(task Task) {
			time.Sleep(5 * time.Second) // Simulate task processing time
			task.Status = "completed"
			task.AssignedTo = ""
			if err := wm.updateTask(task); err != nil {
				log.Printf("Failed to update task %s: %v", task.ID, err)
			}
			wm.TaskRunnerChanged <- true
		}(task)

	}
}

// Work on the task assignment logic for a specific worker
func (wm *WorkerManager) HandleAssignedTasks() {
	// Get all tasks assigned to the worker
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		wm.processAssignedTasks()
	}
}

func (wm *WorkerManager) updateTask(task Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}

	taskPath := path.Join(zkRoot, tasksPath, task.ID)
	_, err = wm.Conn.Set(taskPath, data, -1)
	return err
}
