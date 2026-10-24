package web

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// Run uses process signals, so exercise its actual HTTP server in a child process.
func TestHTTPTimeoutHelper(t *testing.T) {
	addr := os.Getenv("MILLI_TEST_HTTP_ADDRESS")
	if addr == "" {
		return
	}
	s := NewService(Address(addr))
	s.Engine().GET("/ready", func(c *gin.Context) { c.Status(http.StatusOK) })
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
}

func startHTTPService(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHTTPTimeoutHelper$")
	cmd.Env = append(os.Environ(), "MILLI_TEST_HTTP_ADDRESS="+addr)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("HTTP service exited: %v\n%s", err, output.String())
			}
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			t.Errorf("HTTP service failed to stop\n%s", output.String())
		}
	})
	client := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://" + addr + "/ready")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return addr
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("HTTP service did not become ready")
	return ""
}

func TestServerClosesIncompleteHeaders(t *testing.T) {
	addr := startHTTPService(t)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := io.WriteString(c, fmt.Sprintf("GET /ready HTTP/1.1\r\nHost: %s\r\nX-Slow: ", addr)); err != nil {
		t.Fatal(err)
	}
	if err := c.SetReadDeadline(time.Now().Add(7 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, err = http.ReadResponse(bufio.NewReader(c), nil)
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		t.Fatal("server left incomplete HTTP headers open without a deadline")
	}
	if err == nil {
		t.Fatal("incomplete request unexpectedly succeeded")
	}
}
