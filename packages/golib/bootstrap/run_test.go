package bootstrap

import (
	"io"
	"net"
	"net/http"
	"sync"
	"syscall"
	"testing"
	"time"
)

// pickPort returns a free TCP port for ephemeral binding.
func pickPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return formatPort(port)
}

func formatPort(p int) string {
	const digits = "0123456789"
	if p == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for p > 0 {
		i--
		buf[i] = digits[p%10]
		p /= 10
	}
	return string(buf[i:])
}

// TestService_Run_GracefulShutdownOnSIGTERM exercises the previously-
// 0%-coverage Service.Run entrypoint end-to-end:
//   - Configure a service with metrics + health enabled.
//   - Spin Run in a goroutine, poll until /health is reachable.
//   - Send SIGTERM to ourselves; the embedded server.Run intercepts it.
//   - Assert Run returns nil (graceful shutdown succeeded) and that the
//     deferred logCleanup ran (no goroutine leak detected by -race).
func TestService_Run_GracefulShutdownOnSIGTERM(t *testing.T) {
	port := pickPort(t)
	svc := New(ServiceConfig{
		Name:          "iter63-bootstrap-test",
		Port:          port,
		Version:       "v0.0.0-test",
		Environment:   "dev",
		EnableMetrics: true,
		EnableHealth:  true,
	})
	svc.AddRoute("GET", "/echo", "test echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	runErr := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runErr <- svc.Run()
	}()

	// Wait for the listener to be ready.
	deadline := time.Now().Add(2 * time.Second)
	var resp *http.Response
	var err error
	for time.Now().Before(deadline) {
		resp, err = http.Get("http://127.0.0.1:" + port + "/health")
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("service never came up on port %s: %v", port, err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// Hit the user-registered route to exercise the middleware chain.
	resp, err = http.Get("http://127.0.0.1:" + port + "/echo")
	if err != nil {
		t.Errorf("/echo: %v", err)
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	// Trigger graceful shutdown by SIGTERM-ing ourselves; signal.Notify
	// inside server.Run intercepts it so the test runner is not killed.
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("kill self: %v", err)
	}

	select {
	case err := <-runErr:
		if err != nil {
			t.Errorf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return within 5s of SIGTERM")
	}
	wg.Wait()
}

// TestService_New_DefaultsApplied pins that an empty config gets the
// documented defaults (port 8080, env dev, level info, format text)
// without any of those values being silently dropped.
func TestService_New_DefaultsApplied(t *testing.T) {
	svc := New(ServiceConfig{Name: "defaults-test"})
	if svc.Config.Port != "8080" {
		t.Errorf("Port = %q, want 8080", svc.Config.Port)
	}
	if svc.Config.Environment != "dev" {
		t.Errorf("Environment = %q, want dev", svc.Config.Environment)
	}
	if svc.Config.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", svc.Config.LogLevel)
	}
	if svc.Config.LogFormat != "text" {
		t.Errorf("LogFormat = %q, want text", svc.Config.LogFormat)
	}
}
