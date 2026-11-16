package httpserver

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"net/http"
	"testing"
)

type bareTestWriter struct {
	header   http.Header
	statuses []int
	body     bytes.Buffer
}

func newBareTestWriter() *bareTestWriter {
	return &bareTestWriter{
		header: make(http.Header),
	}
}

func (writer *bareTestWriter) Header() http.Header {
	return writer.header
}

func (writer *bareTestWriter) WriteHeader(status int) {
	writer.statuses = append(writer.statuses, status)
}

func (writer *bareTestWriter) Write(body []byte) (int, error) {
	return writer.body.Write(body)
}

type flushingTestWriter struct {
	*bareTestWriter
	flushCalls int
}

func (writer *flushingTestWriter) Flush() {
	writer.flushCalls++
}

type hijackingTestWriter struct {
	*bareTestWriter
	serverConn net.Conn
	clientConn net.Conn
}

func (writer *hijackingTestWriter) Hijack() (
	net.Conn,
	*bufio.ReadWriter,
	error,
) {
	server, client := net.Pipe()

	writer.serverConn = server
	writer.clientConn = client

	buffered := bufio.NewReadWriter(
		bufio.NewReader(server),
		bufio.NewWriter(server),
	)

	return server, buffered, nil
}

type flushHijackTestWriter struct {
	*flushingTestWriter
	hijacker *hijackingTestWriter
}

func (writer *flushHijackTestWriter) Hijack() (
	net.Conn,
	*bufio.ReadWriter,
	error,
) {
	return writer.hijacker.Hijack()
}

type flushErrorTestWriter struct {
	*bareTestWriter
	called bool
}

func (writer *flushErrorTestWriter) FlushError() error {
	writer.called = true
	return nil
}

func TestResponseRecorderPreservesFlusher(t *testing.T) {
	original := &flushingTestWriter{
		bareTestWriter: newBareTestWriter(),
	}

	recorder, wrapped := newResponseRecorder(original)

	flusher, ok := wrapped.(http.Flusher)
	if !ok {
		t.Fatal("expected wrapped writer to implement http.Flusher")
	}

	flusher.Flush()

	if original.flushCalls != 1 {
		t.Errorf("expected one flush call, got %d", original.flushCalls)
	}

	if recorder.status() != http.StatusOK {
		t.Errorf("expected implicit status 200, got %d", recorder.status())
	}

	if recorder.bytesWritten != 0 {
		t.Errorf("expected zero bytes, got %d", recorder.bytesWritten)
	}
}

func TestResponseRecorderDoesNotAdvertiseUnsupportedFlusher(t *testing.T) {
	original := newBareTestWriter()

	_, wrapped := newResponseRecorder(original)

	if _, ok := wrapped.(http.Flusher); ok {
		t.Fatal("wrapper must not implement http.Flusher when original does not")
	}
}

func TestResponseRecorderPreservesHijacker(t *testing.T) {
	original := &hijackingTestWriter{
		bareTestWriter: newBareTestWriter(),
	}

	recorder, wrapped := newResponseRecorder(original)

	hijacker, ok := wrapped.(http.Hijacker)
	if !ok {
		t.Fatal("expected wrapped writer to implement http.Hijacker")
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		t.Fatalf("unexpected hijack error: %v", err)
	}

	if !recorder.hijacked {
		t.Error("expected recorder to track successful hijacking")
	}

	if recorder.status() != http.StatusSwitchingProtocols {
		t.Errorf("expected status 101, got %d", recorder.status())
	}

	_ = conn.Close()
	_ = original.clientConn.Close()
}

func TestResponseRecorderDoesNotAdvertiseUnsupportedHijacker(t *testing.T) {
	original := newBareTestWriter()

	_, wrapped := newResponseRecorder(original)

	if _, ok := wrapped.(http.Hijacker); ok {
		t.Fatal("wrapper must not implement http.Hijacker when original does not")
	}
}

func TestResponseRecorderPreservesBothInterfaces(t *testing.T) {
	hijacker := &hijackingTestWriter{
		bareTestWriter: newBareTestWriter(),
	}

	original := &flushHijackTestWriter{
		flushingTestWriter: &flushingTestWriter{
			bareTestWriter: newBareTestWriter(),
		},
		hijacker: hijacker,
	}

	_, wrapped := newResponseRecorder(original)

	if _, ok := wrapped.(http.Flusher); !ok {
		t.Fatal("expected wrapped writer to preserve http.Flusher")
	}

	if _, ok := wrapped.(http.Hijacker); !ok {
		t.Fatal("expected wrapped writer to preserve http.Hijacker")
	}
}

func TestResponseRecorderUnwrapsOriginalWriter(t *testing.T) {
	original := newBareTestWriter()

	_, wrapped := newResponseRecorder(original)

	unwrapper, ok := wrapped.(interface {
		Unwrap() http.ResponseWriter
	})

	if !ok {
		t.Fatal("expected wrapped writer to expose Unwrap")
	}

	if unwrapper.Unwrap() != original {
		t.Fatal("Unwrap returned a different writer")
	}
}

func TestResponseControllerCanReachOriginalWriter(t *testing.T) {
	original := &flushErrorTestWriter{
		bareTestWriter: newBareTestWriter(),
	}

	_, wrapped := newResponseRecorder(original)

	controller := http.NewResponseController(wrapped)

	if err := controller.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}

	if !original.called {
		t.Error("expected ResponseController to reach original writer")
	}
}

func TestResponseRecorderHandlesInformationalStatus(t *testing.T) {
	original := newBareTestWriter()

	recorder, wrapped := newResponseRecorder(original)

	wrapped.WriteHeader(http.StatusEarlyHints)
	wrapped.WriteHeader(http.StatusOK)

	if recorder.status() != http.StatusOK {
		t.Errorf(
			"expected final status 200, got %d",
			recorder.status(),
		)
	}

	if len(original.statuses) != 2 {
		t.Fatalf("expected two status writes, got %d", len(original.statuses))
	}

	if original.statuses[0] != http.StatusEarlyHints ||
		original.statuses[1] != http.StatusOK {
		t.Errorf("unexpected status sequence: %v", original.statuses)
	}
}

func TestHijackErrorDoesNotMarkRecorderHijacked(t *testing.T) {
	original := &failingHijackerTestWriter{
		bareTestWriter: newBareTestWriter(),
	}

	recorder, wrapped := newResponseRecorder(original)

	hijacker, ok := wrapped.(http.Hijacker)
	if !ok {
		t.Fatal("expected wrapped writer to implement http.Hijacker")
	}

	_, _, err := hijacker.Hijack()

	if !errors.Is(err, errTestHijack) {
		t.Fatalf("expected test hijack error, got %v", err)
	}

	if recorder.hijacked {
		t.Error("failed hijack must not mark recorder as hijacked")
	}
}

var errTestHijack = errors.New("test hijack failure")

type failingHijackerTestWriter struct {
	*bareTestWriter
}

func (writer *failingHijackerTestWriter) Hijack() (
	net.Conn,
	*bufio.ReadWriter,
	error,
) {
	return nil, nil, errTestHijack
}
