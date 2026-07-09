package oauth

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

// NewCallbackHTTPServer listens on listenAddr and serves callback on /callback.
func NewCallbackHTTPServer(listenAddr string, callback http.HandlerFunc) (*http.Server, net.Listener, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", callback)

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to listen on %s: %w", listenAddr, err)
	}

	return &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}, listener, nil
}
