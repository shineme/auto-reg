//go:build windows

package main

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

const jobObjectLimitKillOnJobClose = 0x00002000
const threadSnapshotFlags = 0x00000004

type processJob struct {
	mu     sync.Mutex
	handle windows.Handle
	pid    int
}

func newProcessJob() (*processJob, error) {
	handle, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	_, err = windows.SetInformationJobObject(
		handle,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if err != nil {
		_ = windows.CloseHandle(handle)
		return nil, err
	}
	return &processJob{handle: handle}, nil
}

func (job *processJob) Track(pid int) error {
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.handle == 0 || pid <= 0 {
		return nil
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	if err := windows.AssignProcessToJobObject(job.handle, handle); err != nil {
		return err
	}
	job.pid = pid
	return nil
}

func (job *processJob) Stop() {
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.handle == 0 {
		return
	}
	_ = windows.TerminateJobObject(job.handle, 1)
	_ = windows.CloseHandle(job.handle)
	job.handle = 0
	job.pid = 0
}

func (job *processJob) PID() int {
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.pid
}

func resumeProcess(pid int) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(threadSnapshotFlags, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ThreadEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	resumed := false
	for {
		if err := windows.Thread32First(snapshot, &entry); err != nil {
			return err
		}
		for {
			if entry.OwnerProcessID == uint32(pid) {
				thread, openErr := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
				if openErr == nil {
					_, resumeErr := windows.ResumeThread(thread)
					_ = windows.CloseHandle(thread)
					if resumeErr == nil {
						resumed = true
					}
				}
			}
			entry = windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(entry))}
			err = windows.Thread32Next(snapshot, &entry)
			if err != nil {
				break
			}
		}
		break
	}
	if !resumed {
		return fmt.Errorf("no thread resumed for PID %d", pid)
	}
	return nil
}
