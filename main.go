package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"
)

const TIMEOUT time.Duration = 3 * time.Second

func helloWorld(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Fatal("Parse Error")
	}

	fmt.Fprintf(w, "hello world")
}

func main() {
	http.HandleFunc("/", helloWorld)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           nil,
		TLSConfig:         nil,
		ReadTimeout:       TIMEOUT,
		ReadHeaderTimeout: TIMEOUT,
		WriteTimeout:      TIMEOUT,
		IdleTimeout:       TIMEOUT,
		MaxHeaderBytes:    0,
		TLSNextProto:      map[string]func(*http.Server, *tls.Conn, http.Handler){},
		ConnState:         nil,
		ErrorLog:          &log.Logger{},
		BaseContext:       nil,
		ConnContext:       nil,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Error")
	}
}
