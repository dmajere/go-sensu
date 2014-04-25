package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

type sensuCheckRemote struct {
	Name     string `json:"name"`
	Issued   int64  `json:"issued"`
	Command  string `json:"command"`
	Executed int64  `json:"executed"`
	Output   string `json:"output"`
	Status   uint8  `json:"status"`
	Duration string `json:"duration"`
}

func (s *sensuCheckRemote) Execute() {

	cmd := exec.Command("sh", "-c", s.Command)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	start := time.Now()
	if err := cmd.Start(); err != nil {
		s.Status = 127
		s.Output = err.Error()
	} else {

		if err := cmd.Wait(); err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					s.Status = uint8(status.ExitStatus())
				}
			} else {
				s.Status = 1
				s.Output = err.Error()
			}
		} else {
			s.Status = 0
		}
		s.Executed = start.Unix()
		s.Duration = fmt.Sprintf("%.3f", time.Since(start).Seconds())
		s.Output = string(out.Bytes())
	}
}

type sensuCheck struct {
	// Base Fields
	sensuCheckRemote
	Interval    uint64   `json:"interval"`
	Occurrences int64    `json:"occurrences"`
	Refresh     uint64   `json:"refresh"`
	Handlers    []string `json:"handlers"`
	Subscribers []string `json:"subscribers"`
	Local       bool     `json:"_"`
}

type LocalResult struct {
	Client string      `json:"client"`
	Check  *sensuCheck `json:"check"`
}
type RemoteResult struct {
	Client string            `json:"client"`
	Check  *sensuCheckRemote `json:"check"`
}
