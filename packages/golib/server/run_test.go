package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// pickPort returns a free TCP port for binding test servers.
func pickPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return fmt.Sprintf("%d", port)
}

// TestServer_Run_GracefulShutdownOnSIGTERM proves Server.Run binds, serves
// at least one request, and shuts down cleanly when SIGTERM is delivered.
// This was the 0%-coverage hot path of the public Server entrypoint.
func TestServer_Run_GracefulShutdownOnSIGTERM(t *testing.T) {
	port := pickPort(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	s := New(port, mux)

	runErr := make(chan error, 1)
	go func() { runErr <- s.Run() }()

	// Wait for the listener to be ready (ListenAndServe is async).
	deadline := time.Now().Add(2 * time.Second)
	var resp *http.Response
	var err error
	for time.Now().Before(deadline) {
		resp, err = http.Get("http://127.0.0.1:" + port + "/")
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server never came up: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "ok" {
		t.Errorf("body = %q", body)
	}

	// Send SIGTERM to ourselves; signalctx-style Notify intercepts it so the
	// test runner is not killed.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("kill self: %v", err)
	}

	select {
	case err := <-runErr:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after SIGTERM")
	}
}

// TestServer_Shutdown covers the explicit Shutdown wrapper.
func TestServer_Shutdown(t *testing.T) {
	s := New(pickPort(t), http.NewServeMux())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	// Server has not been started, so Shutdown should return nil immediately.
	if err := s.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown on un-started server returned %v", err)
	}
}

// TestDrainableServer_ListenAndServe binds a real listener and proves the
// drain middleware lets traffic through pre-drain and rejects with 503
// post-drain. Closes the 0% gap on ListenAndServe and 30% gap on Drain.
func TestDrainableServer_ListenAndServe(t *testing.T) {
	port := pickPort(t)

	// Slow handler so we can observe in-flight tracking during drain.
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(150 * time.Millisecond)
		_, _ = w.Write([]byte("done"))
	})
	ds := NewDrainable("127.0.0.1:"+port, slow)

	srvDone := make(chan error, 1)
	go func() {
		err := ds.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			srvDone <- err
		}
		close(srvDone)
	}()

	// Wait for the listener.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.Dial("tcp", "127.0.0.1:"+port)
		if err == nil {
			c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Fire one slow request, then start drain — this exercises the
	// "wait for in-flight to finish" branch of Drain.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := http.Get("http://127.0.0.1:" + port + "/")
		if err != nil {
			t.Errorf("in-flight request failed: %v", err)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if string(body) != "done" {
			t.Errorf("in-flight body = %q", body)
		}
	}()

	// Give the slow request a beat to register inFlight++.
	time.Sleep(30 * time.Millisecond)
	if ds.InFlight() == 0 {
		t.Error("InFlight should be >0 while slow handler is running")
	}

	// Drain returns once in-flight reaches zero.
	if err := ds.Drain(2 * time.Second); err != nil {
		t.Errorf("Drain: %v", err)
	}
	if !ds.IsDraining() {
		t.Error("IsDraining should be true after Drain()")
	}

	wg.Wait()

	// New requests should now be refused with 503 (server is shut down so
	// Dial will fail too — either is acceptable).
	resp, err := http.Get("http://127.0.0.1:" + port + "/")
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("post-drain status = %d, want 503; body=%q", resp.StatusCode, body)
		}
		if !strings.Contains(string(body), "draining") {
			t.Errorf("post-drain body = %q, want substring 'draining'", body)
		}
	}

	select {
	case err := <-srvDone:
		if err != nil {
			t.Errorf("ListenAndServe returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("ListenAndServe did not return after drain")
	}
}
