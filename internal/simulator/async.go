// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package simulator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dotandev/hintents/internal/logger"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

type AsyncJob struct {
	ID          string              `json:"id"`
	Status      JobStatus           `json:"status"`
	SubmittedAt time.Time           `json:"submitted_at"`
	CompletedAt *time.Time          `json:"completed_at,omitempty"`
	Response    *SimulationResponse `json:"response,omitempty"`
	Error       string              `json:"error,omitempty"`
}

type AsyncRunner struct {
	runner RunnerInterface
	jobs   map[string]*AsyncJob
	mu     sync.RWMutex
}

func NewAsyncRunner(runner RunnerInterface) *AsyncRunner {
	return &AsyncRunner{
		runner: runner,
		jobs:   make(map[string]*AsyncJob),
	}
}

func generateJobID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *AsyncRunner) Submit(req *SimulationRequest) (string, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	jobID := generateJobID()
	now := time.Now()
	job := &AsyncJob{
		ID:          jobID,
		Status:      JobStatusPending,
		SubmittedAt: now,
	}

	a.mu.Lock()
	a.jobs[jobID] = job
	a.mu.Unlock()

	logger.Logger.Info("Async simulation submitted", "job_id", jobID)

	go func() {
		a.mu.Lock()
		job.Status = JobStatusRunning
		a.mu.Unlock()

		var simReq SimulationRequest
		if err := json.Unmarshal(reqBytes, &simReq); err != nil {
			a.mu.Lock()
			job.Status = JobStatusFailed
			job.Error = fmt.Sprintf("failed to unmarshal request: %v", err)
			t := time.Now()
			job.CompletedAt = &t
			a.mu.Unlock()
			return
		}

		resp, err := a.runner.Run(context.Background(), &simReq)

		a.mu.Lock()
		t := time.Now()
		job.CompletedAt = &t
		if err != nil {
			job.Status = JobStatusFailed
			job.Error = err.Error()
		} else {
			job.Status = JobStatusCompleted
			job.Response = resp
		}
		a.mu.Unlock()
	}()

	return jobID, nil
}

func (a *AsyncRunner) Poll(jobID string) (*AsyncJob, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	job, exists := a.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job %s not found", jobID)
	}
	return job, nil
}

type PollConfig struct {
	Interval time.Duration
	Timeout  time.Duration
}

func DefaultPollConfig() PollConfig {
	return PollConfig{
		Interval: 500 * time.Millisecond,
		Timeout:  5 * time.Minute,
	}
}

func (a *AsyncRunner) Wait(ctx context.Context, jobID string, cfg PollConfig) (*AsyncJob, error) {
	deadline := time.After(cfg.Timeout)
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for job %s after %v", jobID, cfg.Timeout)
		case <-ticker.C:
			job, err := a.Poll(jobID)
			if err != nil {
				return nil, err
			}
			switch job.Status {
			case JobStatusCompleted, JobStatusFailed:
				return job, nil
			}
		}
	}
}

func (a *AsyncRunner) Cleanup(jobID string) {
	a.mu.Lock()
	delete(a.jobs, jobID)
	a.mu.Unlock()
}
