package server

import (
	"net"
	"net/http"
	"net/http/httptest"
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
	_, port, _ := net.SplitHostPort(addr)
	l.Close()
	return port
}

// TestServer_Run_ListenError pins the errCh-with-non-nil-err branch in
// Run() — Run must return the error if ListenAndServe fails to bind
// (e.g., port already in use), not block waiting for SIGTERM.
func TestServer_Run_ListenError(t *testing.T) {
	// Hold a listener on an OS-assigned port so the second bind collides.
	hold, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Close()

	_, port, _ := net.SplitHostPort(hold.Addr().String())
	s := New(port, http.NewServeMux())
	// Force the server to bind to the same address as `hold`.
	s.srv.Addr = hold.Addr().String()

	done := make(chan error, 1)
	go func() { done <- s.Run() }()

	select {
	case err := <-done:
		if err == nil {
			t.Error("Run returned nil for bind collision")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after bind error")
	}
}

// TestDrain_DrainCompletesViaDrainCh pins the case where a request is
// in-flight when Drain is called and completes naturally — must exit
// via the drainCh signal, not the timeout.
func TestDrain_DrainCompletesViaDrainCh(t *testing.T) {
	hold := make(chan struct{})
	released := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, _ *http.Request) {
		<-hold
		w.WriteHeader(200)
		close(released)
	})

	port := freePort(t)
	ds := NewDrainable(":"+port, mux)
	go ds.ListenAndServe()
	defer ds.server.Close()
	time.Sleep(100 * time.Millisecond) // wait for listen

	// Fire a slow request.
	go func() { http.Get("http://127.0.0.1:" + port + "/slow") }()
	// Wait until the handler is actually running.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && ds.InFlight() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if ds.InFlight() == 0 {
		t.Fatal("InFlight never incremented")
	}

	// Drain in a goroutine; release the handler so drainCh fires.
	drainDone := make(chan error, 1)
	go func() { drainDone <- ds.Drain(5 * time.Second) }()
	time.Sleep(50 * time.Millisecond)
	close(hold)

	select {
	case err := <-drainDone:
		if err != nil {
			t.Errorf("Drain: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Drain did not return after request completed")
	}
	<-released
}

// TestDrain_TimeoutForcesShutdown pins the timer.C branch — when
// requests don't complete in time, Drain forces server.Shutdown.
func TestDrain_TimeoutForcesShutdown(t *testing.T) {
	hold := make(chan struct{})
	defer close(hold)

	mux := http.NewServeMux()
	mux.HandleFunc("/wedged", func(w http.ResponseWriter, _ *http.Request) {
		<-hold
	})
	port := freePort(t)
	ds := NewDrainable(":"+port, mux)
	go ds.ListenAndServe()
	defer ds.server.Close()
	time.Sleep(100 * time.Millisecond)

	go func() { http.Get("http://127.0.0.1:" + port + "/wedged") }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && ds.InFlight() == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	// Drain with a tight timeout — must return without waiting for hold.
	start := time.Now()
	err := ds.Drain(100 * time.Millisecond)
	elapsed := time.Since(start)
	// timer.C fires at 100ms; the inner server.Shutdown then waits up to
	// 5s for the wedged request. The important contract is that Drain
	// did NOT block on the request indefinitely.
	if elapsed > 7*time.Second {
		t.Errorf("Drain took %v, should have timed out via timer.C", elapsed)
	}
	// Either nil or http.ErrServerClosed-equivalent is acceptable; the
	// important thing is the timer.C branch fired and returned.
	_ = err
}

// TestDrainable_ServesNormalThenDrains pins the integration: a request
// goes through DrainMiddleware normally, then a second request after
// Drain() is called gets 503.
func TestDrainable_ServesNormalThenDrains(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})
	ds := NewDrainable(":0", mux)

	// Normal request.
	rec := httptest.NewRecorder()
	ds.server.Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 {
		t.Errorf("normal: %d", rec.Code)
	}

	// After Drain, the middleware should reject.
	ds.draining.Store(true)
	rec2 := httptest.NewRecorder()
	ds.server.Handler.ServeHTTP(rec2, httptest.NewRequest("GET", "/", nil))
	if rec2.Code != http.StatusServiceUnavailable {
		t.Errorf("drained: %d", rec2.Code)
	}
	if rec2.Header().Get("Connection") != "close" || rec2.Header().Get("Retry-After") != "30" {
		t.Errorf("drain headers: %v", rec2.Header())
	}
}
