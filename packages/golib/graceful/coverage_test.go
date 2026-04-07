package graceful

import (
	"context"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

// freePort grabs an OS-assigned free TCP port.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

// instantNotify returns a context that is already cancelled, so the
// server's signal-wait select fires immediately and falls through to
// shutdown without needing real signal delivery.
func instantNotify() func(context.Context, ...os.Signal) (context.Context, context.CancelFunc) {
	return func(parent context.Context, _ ...os.Signal) (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(parent)
		// Fire on next scheduler tick — gives the server goroutine a beat
		// to call ListenAndServe before we cancel.
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		return ctx, cancel
	}
}

// TestListenAndServe_FullLifecycle pins the OnReady → drain → OnShutdown
// → shutdown chain end-to-end. Covers ListenAndServe, listenAndServe, and
// the shutdown helper including DrainDelay and OnShutdown branches.
func TestListenAndServe_FullLifecycle(t *testing.T) {
	var ready, down atomic.Bool
	addr := freePort(t)
	s := New(Config{
		Addr:            addr,
		Handler:         http.NewServeMux(),
		ShutdownTimeout: 2 * time.Second,
		DrainDelay:      20 * time.Millisecond,
		OnReady:         func() { ready.Store(true) },
		OnShutdown:      func() { down.Store(true) },
	})

	errCh := make(chan error, 1)
	go func() { errCh <- s.listenAndServe(instantNotify()) }()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("listenAndServe: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("listenAndServe did not return within 5s")
	}

	if !ready.Load() {
		t.Error("OnReady not called")
	}
	if !down.Load() {
		t.Error("OnShutdown not called")
	}
	if s.State() != StateStopped {
		t.Errorf("final state = %v, want stopped", s.State())
	}
}

// TestListenAndServe_PublicEntry covers the one-line ListenAndServe()
// wrapper by sending SIGTERM to ourselves once the server is up. The
// signal handler installed by signal.NotifyContext intercepts it so the
// test process is not killed.
func TestListenAndServe_PublicEntry(t *testing.T) {
	s := New(Config{
		Addr:            freePort(t),
		Handler:         http.NewServeMux(),
		ShutdownTimeout: 2 * time.Second,
	})
	errCh := make(chan error, 1)
	go func() { errCh <- s.ListenAndServe() }()

	// Give it a beat to install the signal handler and start listening.
	time.Sleep(100 * time.Millisecond)
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("ListenAndServe: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ListenAndServe did not return after SIGTERM")
	}
}

// TestShutdown_DirectCallReturnsNoError pins the public Shutdown() entry.
func TestShutdown_DirectCall(t *testing.T) {
	s := New(Config{Addr: freePort(t), Handler: http.NewServeMux()})
	// http.Server.Shutdown on a never-started server returns nil.
	if err := s.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown on idle server: %v", err)
	}
}

// TestState_StringAllValues pins the four named state strings plus the
// "unknown" default branch.
func TestState_StringAllValues(t *testing.T) {
	cases := map[State]string{
		StateStarting: "starting",
		StateReady:    "ready",
		StateDraining: "draining",
		StateStopped:  "stopped",
		State(99):     "unknown",
	}
	for st, want := range cases {
		if got := st.String(); got != want {
			t.Errorf("State(%d).String() = %q, want %q", st, got, want)
		}
	}
}

// TestIsReady pins the IsReady predicate across state transitions.
func TestIsReady_Transitions(t *testing.T) {
	s := New(Config{Addr: freePort(t), Handler: http.NewServeMux()})
	if s.IsReady() {
		t.Error("IsReady should be false in StateStarting")
	}
	s.state.Store(int32(StateReady))
	if !s.IsReady() {
		t.Error("IsReady should be true in StateReady")
	}
	s.state.Store(int32(StateStopped))
	if s.IsReady() {
		t.Error("IsReady should be false in StateStopped")
	}
}
