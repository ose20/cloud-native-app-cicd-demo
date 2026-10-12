// dummy version comment: v0.1.2
package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"
)

const TIMEOUT time.Duration = 3 * time.Second

var version = "dev"

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

func main() {
	http.HandleFunc("/", helloWorld)
	http.HandleFunc("/error", errorResponse)

	server := &http.Server{
		Addr:                         ":8080",
		Handler:                      nil,
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
