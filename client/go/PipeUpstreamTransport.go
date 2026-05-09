package modcdp

import (
	"fmt"
	"os"
	"sync"
)

type PipeUpstreamTransport struct {
	UpstreamTransport
	URL       string
	PipeRead  *os.File
	PipeWrite *os.File
	writeMu   sync.Mutex
	closed    bool
}

func NewPipeUpstreamTransport(pipe_read *os.File, pipe_write *os.File, url string) *PipeUpstreamTransport {
	if url == "" {
		url = "pipe://unknown"
	}
	return &PipeUpstreamTransport{URL: url, PipeRead: pipe_read, PipeWrite: pipe_write}
}

func (t *PipeUpstreamTransport) Update(config map[string]any) {
	if config == nil {
		return
	}
	if pipeRead, _ := config["pipe_read"].(*os.File); pipeRead != nil {
		t.PipeRead = pipeRead
	}
	if pipeWrite, _ := config["pipe_write"].(*os.File); pipeWrite != nil {
		t.PipeWrite = pipeWrite
	}
	if cdpURL, _ := config["cdp_url"].(string); cdpURL != "" {
		t.URL = cdpURL
	} else if rawURL, _ := config["url"].(string); rawURL != "" {
		t.URL = rawURL
	}
}

func (t *PipeUpstreamTransport) GetLauncherConfig() LaunchOptions {
	return LaunchOptions{RemoteDebugging: "pipe"}
}

func (t *PipeUpstreamTransport) Connect() error {
	if t.PipeRead == nil || t.PipeWrite == nil {
		return fmt.Errorf("upstream.mode=pipe requires launcher-provided pipe_read and pipe_write handles")
	}
	t.closed = false
	go t.readLoop()
	return nil
}

func (t *PipeUpstreamTransport) Send(message map[string]any) error {
	if t.PipeWrite == nil || t.closed {
		return fmt.Errorf("CDP pipe is not connected")
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	return writePipeMessage(t.PipeWrite, message)
}

func (t *PipeUpstreamTransport) Close() error {
	t.closed = true
	if t.PipeRead != nil {
		_ = t.PipeRead.Close()
	}
	if t.PipeWrite != nil {
		_ = t.PipeWrite.Close()
	}
	return nil
}

func (t *PipeUpstreamTransport) readLoop() {
	for {
		message, err := readPipeMessage(t.PipeRead)
		if err != nil {
			if !t.closed {
				t.emitClose(err)
			}
			return
		}
		t.emitRecv(message)
	}
}
