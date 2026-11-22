// dummy version comment: v0.1.3
package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const TIMEOUT time.Duration = 3 * time.Second

var version = "dev"

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hello_world_http_requests_total",
			Help: "Total number of HTTP requests handled by the application.",
		},
		[]string{"handler", "method", "status"},
	)
	httpRequestDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hello_world_http_request_duration_seconds",
			Help:    "Duration of HTTP requests handled by the application, in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"handler", "method"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDurationSeconds)
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
	headerSent bool
}

func (recorder *statusRecorder) WriteHeader(statusCode int) {
	if recorder.headerSent {
		return
	}
	recorder.headerSent = true
	recorder.statusCode = statusCode
	recorder.ResponseWriter.WriteHeader(statusCode)
}

func (recorder *statusRecorder) Write(body []byte) (int, error) {
	if !recorder.headerSent {
		recorder.WriteHeader(http.StatusOK)
	}
	return recorder.ResponseWriter.Write(body)
}

func normalizeMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace:
		return method
	default:
		return "OTHER"
	}
}

func instrumentHandler(handlerName string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{
			ResponseWriter: responseWriter,
			statusCode:     http.StatusOK,
		}
		method := normalizeMethod(request.Method)

		handler.ServeHTTP(recorder, request)

		httpRequestsTotal.WithLabelValues(
			handlerName,
			method,
			strconv.Itoa(recorder.statusCode),
		).Inc()
		httpRequestDurationSeconds.WithLabelValues(handlerName, method).
			Observe(time.Since(startedAt).Seconds())
	})
}

func helloWorld(responseWriter http.ResponseWriter, request *http.Request) {
	err := request.ParseForm()
	if err != nil {
		log.Fatal("Parse Error")
	}

	fmt.Fprintf(responseWriter, "hello world: %s", version)
}

func errorResponse(responseWriter http.ResponseWriter, _ *http.Request) {
	http.Error(responseWriter, "intentional failure", http.StatusServiceUnavailable)
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", instrumentHandler("root", http.HandlerFunc(helloWorld)))
	mux.Handle("/error", instrumentHandler("error", http.HandlerFunc(errorResponse)))
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}

func main() {
	server := &http.Server{
		Addr:                         ":8080",
		Handler:                      newHandler(),
		DisableGeneralOptionsHandler: false,
		TLSConfig:                    nil,
		ReadTimeout:                  TIMEOUT,
		ReadHeaderTimeout:            TIMEOUT,
		WriteTimeout:                 TIMEOUT,
		IdleTimeout:                  TIMEOUT,
		MaxHeaderBytes:               0,
		MaxHeaderValueCount:          0,
		TLSNextProto:                 map[string]func(*http.Server, *tls.Conn, http.Handler){},
		ConnState:                    nil,
		ErrorLog:                     &log.Logger{},
		BaseContext:                  nil,
		ConnContext:                  nil,
		HTTP2:                        nil,
		Protocols:                    nil,
		DisableClientPriority:        false,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("Error")
	}
}
