package unwrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

type Unwrapper struct {
	host        string
	description string
	httpClient  *http.Client
}

func New(host, description, upstreamDNSIPPort string) *Unwrapper {
	// Setup dialer to use upstream DNS
	dialer := &net.Dialer{
		Resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Duration(5000) * time.Millisecond,
				}
				return d.DialContext(ctx, "udp", upstreamDNSIPPort)
			},
		},
	}

	transport := http.DefaultTransport.(*http.Transport)
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, addr)
	}

	client := http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects
			return http.ErrUseLastResponse
		},
		Timeout: 3 * time.Second,
	}

	return &Unwrapper{
		host:        host,
		description: description,
		httpClient:  &client,
	}
}

func (c *Unwrapper) Host() string {
	return c.host
}

func (c *Unwrapper) Description() string {
	return c.description
}

// Do will perform a HEAD request for the given host and path, and check for the
// Location header, if this exists the url contained within will be returned.
func (c *Unwrapper) Do(path string) (*url.URL, *url.URL, error) {
	endpointStr := fmt.Sprintf("https://%s/%s", c.host, path)
	endpoint, _ := url.Parse(endpointStr)
	log.Printf("visiting: %s", endpoint.String())

	// Testing the new HTTP client with the custom DNS resolver.
	resp, err := c.httpClient.Head(endpoint.String())
	if err != nil {
		log.Printf("error: failed doing HEAD on: %s err: %s", endpoint, err)
		return endpoint, nil, err
	}

	location, ok := resp.Header["Location"]

	if !ok {
		log.Printf("error: got nil location header from: %s", endpoint.String())
		return endpoint, nil, errors.New("no location header found")
	} else if len(location) == 0 {
		log.Printf("error: got empty location header from: %s", endpoint.String())
		return endpoint, nil, errors.New("location header empty")
	}

	out, err := url.Parse(location[0])
	if err != nil {
		log.Printf("error: failed to parse location url: %s from: %s", location[0], endpoint.String())
		return endpoint, nil, errors.New("error parsing location url")
	}
	return endpoint, out, nil
}
