//go:build !windows

package main

type processJob struct{}

func newProcessJob() (*processJob, error) { return &processJob{}, nil }
func (job *processJob) Track(int) error   { return nil }
func (job *processJob) Stop()             {}
func (job *processJob) PID() int          { return 0 }
