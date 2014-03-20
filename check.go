
package main

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
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

	start := time.Now()
	command := strings.Split(s.Command, " ")
	out, err := exec.Command(command[0], command[1:]...).CombinedOutput()
	log.Println(out, err)
	s.Executed = start.Unix()
	s.Duration = fmt.Sprintf("%.3f", time.Since(start).Seconds())
	s.Status = 0
	s.Output = string(out)
	if err != nil {
		s.Status = 1
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

type Message struct {
	exchange string
	body     []byte
}
type LocalResult struct {
	Client string      `json:"client"`
	Check  *sensuCheck `json:"check"`
}
type RemoteResult struct {
	Client string            `json:"client"`
	Check  *sensuCheckRemote `json:"check"`
}