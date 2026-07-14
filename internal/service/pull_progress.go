package service

import (
	"bufio"
	"encoding/json"
	"io"
)

type PullEvent struct {
	Type     string `json:"type"`
	Status   string `json:"status"`
	Progress string `json:"progress,omitempty"`
	ID       string `json:"id,omitempty"`
	Error    string `json:"error,omitempty"`
	Message  string `json:"message,omitempty"`
	Current  int64  `json:"current,omitempty"`
	Total    int64  `json:"total,omitempty"`
}

type dockerPullLine struct {
	Status         string `json:"status"`
	ProgressDetail struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	} `json:"progressDetail"`
	Progress string `json:"progress"`
	ID       string `json:"id"`
	Error    string `json:"error"`
}

func ParsePullStream(r io.ReadCloser, ch chan<- PullEvent) {
	defer r.Close()
	defer close(ch)

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		var line dockerPullLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.Error != "" {
			ch <- PullEvent{Type: "error", Error: line.Error}
			return
		}
		ch <- PullEvent{
			Type:     "progress",
			Status:   line.Status,
			Progress: line.Progress,
			ID:       line.ID,
			Current:  line.ProgressDetail.Current,
			Total:    line.ProgressDetail.Total,
		}
	}
}
