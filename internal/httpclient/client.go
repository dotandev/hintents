package httpclient

import "net/http"

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Wrapper struct {
	client *http.Client
}

func New(client *http.Client) HTTPClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &Wrapper{client: client}
}

func (w *Wrapper) Do(req *http.Request) (*http.Response, error) {
	return w.client.Do(req)
}

func (w *Wrapper) Unwrap() *http.Client {
	return w.client
}