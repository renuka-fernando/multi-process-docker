package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Process represents a managed process
type Process struct {
	Name    string
	Command string
	Args    []string
	// If true, this process must start successfully before starting the next process
	Critical bool
	// Restart delay after failure
	RestartDelay time.Duration
}

// ProcessManager manages multiple processes with restart capabilities
type ProcessManager struct {
	processes []*Process
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex
	running   map[string]*exec.Cmd
}

// NewProcessManager creates a new process manager
func NewProcessManager(processes []*Process) *ProcessManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProcessManager{
		processes: processes,
		ctx:       ctx,
		cancel:    cancel,
		running:   make(map[string]*exec.Cmd),
	}
}

// Start begins managing all processes
func (pm *ProcessManager) Start() error {
	log.Println("Process Manager starting...")

	// Start processes in order
	for _, proc := range pm.processes {
		if err := pm.startProcess(proc, true); err != nil {
			if proc.Critical {
				return fmt.Errorf("failed to start critical process %s: %w", proc.Name, err)
			}
			log.Printf("Warning: failed to start process %s: %v", proc.Name, err)
		}

		// If critical, wait a bit to ensure it's stable
		if proc.Critical {
			time.Sleep(1 * time.Second)
		}
	}

	log.Println("All processes started successfully")
	return nil
}

// startProcess starts a single process and monitors it
func (pm *ProcessManager) startProcess(proc *Process, initial bool) error {
	pm.wg.Add(1)

	go func() {
		defer pm.wg.Done()

		for {
			select {
			case <-pm.ctx.Done():
				log.Printf("Process %s: shutdown requested", proc.Name)
				return
			default:
			}

			log.Printf("Starting process: %s", proc.Name)

			cmd := exec.CommandContext(pm.ctx, proc.Command, proc.Args...)
			cmd.Stdout = &prefixedWriter{prefix: fmt.Sprintf("[%s] ", proc.Name), dest: os.Stdout}
			cmd.Stderr = &prefixedWriter{prefix: fmt.Sprintf("[%s] ", proc.Name), dest: os.Stderr}

			// Store the running command
			pm.mu.Lock()
			pm.running[proc.Name] = cmd
			pm.mu.Unlock()

			if err := cmd.Start(); err != nil {
				log.Printf("Process %s: failed to start: %v", proc.Name, err)

				pm.mu.Lock()
				delete(pm.running, proc.Name)
				pm.mu.Unlock()

				if initial {
					return
				}

				// Wait before restarting
				delay := proc.RestartDelay
				if delay == 0 {
					delay = 5 * time.Second
				}

				select {
				case <-time.After(delay):
					continue
				case <-pm.ctx.Done():
					return
				}
			}

			log.Printf("Process %s started with PID: %d", proc.Name, cmd.Process.Pid)

			// Wait for process to complete
			err := cmd.Wait()

			pm.mu.Lock()
			delete(pm.running, proc.Name)
			pm.mu.Unlock()

			// Check if shutdown was requested
			select {
			case <-pm.ctx.Done():
				log.Printf("Process %s: exited during shutdown", proc.Name)
				return
			default:
			}

			if err != nil {
				log.Printf("Process %s: exited with error: %v", proc.Name, err)
			} else {
				log.Printf("Process %s: exited normally", proc.Name)
			}

			// Restart after delay
			delay := proc.RestartDelay
			if delay == 0 {
				delay = 5 * time.Second
			}

			log.Printf("Process %s: restarting in %v...", proc.Name, delay)

			select {
			case <-time.After(delay):
				// Continue to restart
			case <-pm.ctx.Done():
				return
			}
		}
	}()

	// Don't return immediately on first start
	if initial {
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// Shutdown gracefully shuts down all processes
func (pm *ProcessManager) Shutdown() {
	log.Println("Process Manager: initiating graceful shutdown...")

	// Cancel context to stop restart loops
	pm.cancel()

	// Send SIGTERM to all running processes
	pm.mu.Lock()
	for name, cmd := range pm.running {
		if cmd.Process != nil {
			log.Printf("Sending SIGTERM to process: %s (PID: %d)", name, cmd.Process.Pid)
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				log.Printf("Failed to send SIGTERM to %s: %v", name, err)
			}
		}
	}
	pm.mu.Unlock()

	// Wait for processes to exit gracefully (with timeout)
	done := make(chan struct{})
	go func() {
		pm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All processes exited gracefully")
	case <-time.After(30 * time.Second):
		log.Println("Timeout waiting for processes to exit, forcing shutdown...")

		// Force kill remaining processes
		pm.mu.Lock()
		for name, cmd := range pm.running {
			if cmd.Process != nil {
				log.Printf("Force killing process: %s (PID: %d)", name, cmd.Process.Pid)
				cmd.Process.Kill()
			}
		}
		pm.mu.Unlock()
	}

	log.Println("Process Manager shutdown complete")
}

// Wait blocks until shutdown is complete
func (pm *ProcessManager) Wait() {
	pm.wg.Wait()
}

// prefixedWriter adds a prefix to each line written
type prefixedWriter struct {
	prefix string
	dest   *os.File
	buffer []byte
}

func (pw *prefixedWriter) Write(p []byte) (n int, err error) {
	// We need to return len(p) to the caller, not the length of what we actually wrote
	// Otherwise the caller thinks there was an error
	originalLen := len(p)

	// Append incoming data to buffer
	pw.buffer = append(pw.buffer, p...)

	// Process complete lines
	for {
		lineEnd := -1
		for i, b := range pw.buffer {
			if b == '\n' {
				lineEnd = i
				break
			}
		}

		if lineEnd == -1 {
			// No complete line yet, keep buffering
			break
		}

		// Write the line with prefix
		line := pw.buffer[:lineEnd+1]
		prefixed := append([]byte(pw.prefix), line...)

		if _, err := pw.dest.Write(prefixed); err != nil {
			// Even if we fail to write, we should return the original length
			// to avoid breaking the pipe on the caller's side
			return originalLen, nil
		}

		// Remove the processed line from buffer
		pw.buffer = pw.buffer[lineEnd+1:]
	}

	// Return the original length to satisfy the caller
	return originalLen, nil
}

func main() {
	// Define the processes to manage
	processes := []*Process{
		{
			Name:         "grpc-server",
			Command:      "/app/server",
			Args:         []string{},
			Critical:     true, // Server must start first
			RestartDelay: 5 * time.Second,
		},
		{
			Name:         "grpc-client",
			Command:      "/app/client",
			Args:         []string{},
			Critical:     false,
			RestartDelay: 5 * time.Second,
		},
	}

	// Create process manager
	pm := NewProcessManager(processes)

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start all processes
	if err := pm.Start(); err != nil {
		log.Fatalf("Failed to start processes: %v", err)
	}

	// Wait for shutdown signal
	sig := <-sigChan
	log.Printf("Received signal: %v", sig)

	// Shutdown gracefully
	pm.Shutdown()

	log.Println("Process Manager exited")
}
