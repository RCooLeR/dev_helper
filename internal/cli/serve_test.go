package cli

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

type trackedListener struct {
	net.Listener
	closed chan struct{}
	once   sync.Once
}

func (l *trackedListener) Close() error {
	err := l.Listener.Close()
	l.once.Do(func() { close(l.closed) })
	return err
}

func TestServeHTTPDrainsActiveRequestsOnCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tracked := &trackedListener{Listener: listener, closed: make(chan struct{})}
	t.Cleanup(func() { _ = tracked.Close() })
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	finishRequest := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(finishRequest)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "finished")
	})}
	t.Cleanup(func() { _ = server.Close() })
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	serveDone := make(chan error, 1)
	go func() { serveDone <- serveHTTP(ctx, server, tracked) }()

	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{}}
	t.Cleanup(client.CloseIdleConnections)
	requestDone := make(chan error, 1)
	go func() {
		res, err := client.Get("http://" + listener.Addr().String())
		if err != nil {
			requestDone <- err
			return
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err == nil && string(body) != "finished" {
			err = errors.New("request was truncated during shutdown")
		}
		requestDone <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("HTTP request did not reach the server")
	}
	cancel()
	select {
	case <-tracked.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not stop the listener")
	}
	select {
	case err := <-serveDone:
		t.Fatalf("server returned before its active request completed: %v", err)
	default:
	}
	finishRequest()
	select {
	case err := <-requestDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("active request did not finish")
	}
	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("graceful shutdown returned an error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not finish shutdown")
	}
}

func TestServeHTTPClosesListenerWhenAlreadyCanceled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := serveHTTP(ctx, &http.Server{}, listener); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
	if conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second); err == nil {
		conn.Close()
		t.Fatal("listener remained open after cancellation")
	}
}
