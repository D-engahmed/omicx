package httpserver

import (
	"bufio"
	"net"
	"net/http"
)

// responseRecorder tracks the HTTP response status and body size.
type responseRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
	hijacked     bool
}

// newResponseRecorder creates a recorder and preserves optional
// interfaces supported by the original ResponseWriter.
func newResponseRecorder(
	writer http.ResponseWriter,
) (*responseRecorder, http.ResponseWriter) {
	recorder := &responseRecorder{
		ResponseWriter: writer,
	}

	flusher, supportsFlush := writer.(http.Flusher)
	hijacker, supportsHijack := writer.(http.Hijacker)

	switch {
	case supportsFlush && supportsHijack:
		return recorder, &flushHijackResponseWriter{
			responseRecorder: recorder,
			flusher:          flusher,
			hijacker:         hijacker,
		}

	case supportsFlush:
		return recorder, &flushResponseWriter{
			responseRecorder: recorder,
			flusher:          flusher,
		}

	case supportsHijack:
		return recorder, &hijackResponseWriter{
			responseRecorder: recorder,
			hijacker:         hijacker,
		}

	default:
		return recorder, recorder
	}
}

// WriteHeader records the final HTTP response status.
// Informational responses (except 101 Switching Protocols) do not
// prevent a later final response status from being recorded.
func (recorder *responseRecorder) WriteHeader(statusCode int) {
	if recorder.statusCode != 0 {
		return
	}

	if statusCode >= 100 && statusCode < 200 &&
		statusCode != http.StatusSwitchingProtocols {
		recorder.ResponseWriter.WriteHeader(statusCode)
		return
	}

	recorder.statusCode = statusCode
	recorder.ResponseWriter.WriteHeader(statusCode)
}

// Write records the number of response bytes successfully written.
func (recorder *responseRecorder) Write(body []byte) (int, error) {
	if recorder.statusCode == 0 {
		recorder.WriteHeader(http.StatusOK)
	}

	n, err := recorder.ResponseWriter.Write(body)
	recorder.bytesWritten += n

	return n, err
}

// status returns the final response status.
func (recorder *responseRecorder) status() int {
	if recorder.statusCode != 0 {
		return recorder.statusCode
	}

	if recorder.hijacked {
		return http.StatusSwitchingProtocols
	}

	return http.StatusOK
}

// Unwrap allows http.ResponseController to access the original writer.
func (recorder *responseRecorder) Unwrap() http.ResponseWriter {
	return recorder.ResponseWriter
}

// flushResponseWriter preserves http.Flusher.
type flushResponseWriter struct {
	*responseRecorder
	flusher http.Flusher
}

func (writer *flushResponseWriter) Flush() {
	if writer.statusCode == 0 {
		writer.WriteHeader(http.StatusOK)
	}

	writer.flusher.Flush()
}

// hijackResponseWriter preserves http.Hijacker.
type hijackResponseWriter struct {
	*responseRecorder
	hijacker http.Hijacker
}

func (writer *hijackResponseWriter) Hijack() (
	net.Conn,
	*bufio.ReadWriter,
	error,
) {
	conn, buffered, err := writer.hijacker.Hijack()

	if err == nil {
		writer.hijacked = true
	}

	return conn, buffered, err
}

// flushHijackResponseWriter preserves both optional interfaces.
type flushHijackResponseWriter struct {
	*responseRecorder
	flusher  http.Flusher
	hijacker http.Hijacker
}

func (writer *flushHijackResponseWriter) Flush() {
	if writer.statusCode == 0 {
		writer.WriteHeader(http.StatusOK)
	}

	writer.flusher.Flush()
}

func (writer *flushHijackResponseWriter) Hijack() (
	net.Conn,
	*bufio.ReadWriter,
	error,
) {
	conn, buffered, err := writer.hijacker.Hijack()

	if err == nil {
		writer.hijacked = true
	}

	return conn, buffered, err
}
