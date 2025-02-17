package main

import (
	"DistributedTaskSchedulerZK/workers"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//TIP <p>To run your code, right-click the code and select <b>Run</b>.</p> <p>Alternatively, click
// the <icon src="AllIcons.Actions.Execute"/> icon in the gutter and select the <b>Run</b> menu item from here.</p>

func main() {
	worker := workers.Worker{
		ID:   fmt.Sprintf("worker-%d", time.Now().UnixNano()),
		Host: "localhost",
		Port: 8080,
	}
	// Connect to ZooKeeper ensemble
	zkHosts := []string{"localhost:2181"}
	wm, err := workers.NewWorkerManager(zkHosts, worker)
	if err != nil {
		log.Fatalf("Failed to initialize worker manager: %v", err)
	}

	// Register worker
	if err := wm.Register(); err != nil {
		log.Fatalf("Failed to register worker: %v", err)
	}

	if err := wm.CheckNodeStatus(); err != nil {
		log.Fatalf("Failed to check node status: %v", err)
	}

	// Create a new task
	task := workers.Task{
		ID:          "task-7", // Unique task ID
		Description: "Sample task description",
		Status:      "pending",
		AssignedTo:  "",
		CreatedAt:   time.Now(),
	}

	// Add the task to ZooKeeper
	err = wm.AddTask(task)
	if err != nil {
		log.Printf("Failed to add task: %v", err)
	} else {
		log.Println("Task added successfully.")
	}

	// Start leader election
	go wm.ElectLeader()
	go wm.HandleAssignedTasks()

	// Goroutine to listen for changes

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Cleanup
	wm.Conn.Close()
}

/*
TODO:
1. Test with multiple nodes
2. Test reassignment of leader in case of leader failure
*/
