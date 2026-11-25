// Copyright 2026 Erst Users
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestClientConfiguration(t *testing.T) {
	if Client.Timeout != 30*time.Second {
		t.Errorf("Client.Timeout = %s, want %s", Client.Timeout, 30*time.Second)
	}

	transport, ok := Client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Client.Transport has type %T, want *http.Transport", Client.Transport)
	}
	if transport.Proxy == nil {
		t.Fatal("Client.Transport.Proxy is nil")
	}
	if got, want := reflect.ValueOf(transport.Proxy).Pointer(), reflect.ValueOf(http.ProxyFromEnvironment).Pointer(); got != want {
		t.Errorf("Client.Transport.Proxy does not use http.ProxyFromEnvironment")
	}
	if !transport.ForceAttemptHTTP2 {
		t.Error("Client.Transport.ForceAttemptHTTP2 = false, want true")
	}
	if transport.MaxIdleConnsPerHost != 20 {
		t.Errorf("Client.Transport.MaxIdleConnsPerHost = %d, want 20", transport.MaxIdleConnsPerHost)
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("Client.Transport.TLSHandshakeTimeout = %s, want %s", transport.TLSHandshakeTimeout, 10*time.Second)
	}
}

func TestClientRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()

	response, err := Client.Get(server.URL)
	if err != nil {
		t.Fatalf("Client.Get() error = %v", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if got, want := string(body), "ok"; got != want {
		t.Errorf("response body = %q, want %q", got, want)
	}
}

func TestClientRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = io.WriteString(w, "too late")
	}))
	defer server.Close()

	client := *Client
	client.Timeout = 50 * time.Millisecond

	response, err := client.Get(server.URL)
	if err == nil {
		response.Body.Close()
		t.Fatal("Client.Get() succeeded, want timeout error")
	}
	var timeoutErr net.Error
	if !errors.As(err, &timeoutErr) || !timeoutErr.Timeout() {
		t.Errorf("Client.Get() error = %v, want timeout error", err)
	}
}
