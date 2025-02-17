package springcloud

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/textproto"
	"sync/atomic"
	"time"

	"github.com/libgox/gocollections/syncx"
)

const (
	HeaderAccept      = "Accept"
	HeaderContentType = "Content-Type"
)

const (
	MediaJson = "application/json"
	MediaXml  = "application/xml"
)

type ClientConfig struct {
	Discovery Discovery
	// TlsConfig configuration information for tls.
	TlsConfig *tls.Config
	// Timeout default 30s
	Timeout time.Duration
	// ConnectTimeout default 10s
	ConnectTimeout time.Duration
	// Logger structured logger for logging operations
	Logger *slog.Logger
}

type Client struct {
	discovery  Discovery
	httpClient *http.Client
	tlsConfig  *tls.Config
	rrIndices  syncx.Map[string, *atomic.Uint32]
	logger     *slog.Logger
}

func NewClient(config ClientConfig) *Client {
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.ConnectTimeout <= 0 {
		config.ConnectTimeout = 10 * time.Second
	}

	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: config.ConnectTimeout,
			}).DialContext,
			TLSClientConfig: config.TlsConfig,
		},
	}

	c := &Client{
		discovery:  config.Discovery,
		httpClient: httpClient,
		tlsConfig:  config.TlsConfig,
	}

	if config.Logger != nil {
		c.logger = config.Logger
	} else {
		c.logger = slog.Default()
	}

	return c
}

// JsonGet sends a GET request and automatically handles JSON response unmarshalling
func (c *Client) JsonGet(ctx context.Context, serviceName, path string, headers textproto.MIMEHeader, respObj any) error {
	return c.JsonRequest(ctx, serviceName, http.MethodGet, path, nil, headers, respObj)
}

// JsonPost sends a POST request with JSON marshalling of the request body and JSON unmarshalling of the response
func (c *Client) JsonPost(ctx context.Context, serviceName, path string, reqObj any, headers textproto.MIMEHeader, respObj any) error {
	body, err := json.Marshal(reqObj)
	if err != nil {
		return fmt.Errorf("failed to marshal request object: %v", err)
	}
	return c.JsonRequest(ctx, serviceName, http.MethodPost, path, body, headers, respObj)
}

// JsonPut sends a PUT request with JSON marshalling of the request body and JSON unmarshalling of the response
func (c *Client) JsonPut(ctx context.Context, serviceName, path string, reqObj any, headers textproto.MIMEHeader, respObj any) error {
	body, err := json.Marshal(reqObj)
	if err != nil {
		return fmt.Errorf("failed to marshal request object: %v", err)
	}
	return c.JsonRequest(ctx, serviceName, http.MethodPut, path, body, headers, respObj)
}

// JsonDelete sends a DELETE request and automatically handles JSON response unmarshalling
func (c *Client) JsonDelete(ctx context.Context, serviceName, path string, headers textproto.MIMEHeader) error {
	return c.JsonRequest(ctx, serviceName, http.MethodDelete, path, nil, headers, nil)
}

// JsonRequest handles making a request, sending JSON data, and automatically unmarshalling the JSON response
func (c *Client) JsonRequest(ctx context.Context, serviceName, method, path string, body []byte, headers textproto.MIMEHeader, respObj any) error {
	if headers == nil {
		headers = make(textproto.MIMEHeader)
	}

	if headers.Get(HeaderAccept) == "" {
		headers.Set(HeaderAccept, MediaJson)
	}

	if headers.Get(HeaderContentType) == "" {
		headers.Set(HeaderContentType, MediaJson)
	}

	resp, err := c.Request(ctx, serviceName, method, path, body, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if respObj != nil {
			if err := json.NewDecoder(resp.Body).Decode(respObj); err != nil {
				return fmt.Errorf("failed to decode JSON response: %v", err)
			}
		}
	} else {
		responseBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return NewHttpStatusError(resp.StatusCode, fmt.Sprintf("failed to read response body: %v", readErr))
		}

		return NewHttpStatusError(resp.StatusCode, string(responseBody))
	}

	return nil
}

// XmlGet sends a GET request and automatically handles XML response unmarshalling
func (c *Client) XmlGet(ctx context.Context, serviceName, path string, headers textproto.MIMEHeader, respObj any) error {
	return c.XmlRequest(ctx, serviceName, http.MethodGet, path, nil, headers, respObj)
}

// XmlPost sends a POST request with XML marshalling of the request body and XML unmarshalling of the response
func (c *Client) XmlPost(ctx context.Context, serviceName, path string, reqObj any, headers textproto.MIMEHeader, respObj any) error {
	body, err := xml.Marshal(reqObj)
	if err != nil {
		return fmt.Errorf("failed to marshal request object: %v", err)
	}
	return c.XmlRequest(ctx, serviceName, http.MethodPost, path, body, headers, respObj)
}

// XmlPut sends a PUT request with XML marshalling of the request body and XML unmarshalling of the response
func (c *Client) XmlPut(ctx context.Context, serviceName, path string, reqObj any, headers textproto.MIMEHeader, respObj any) error {
	body, err := xml.Marshal(reqObj)
	if err != nil {
		return fmt.Errorf("failed to marshal request object: %v", err)
	}
	return c.XmlRequest(ctx, serviceName, http.MethodPut, path, body, headers, respObj)
}

// XmlDelete sends a DELETE request and automatically handles XML response unmarshalling
func (c *Client) XmlDelete(ctx context.Context, serviceName, path string, headers textproto.MIMEHeader) error {
	return c.XmlRequest(ctx, serviceName, http.MethodDelete, path, nil, headers, nil)
}

// XmlRequest handles making a request, sending XML data, and automatically unmarshalling the XML response
func (c *Client) XmlRequest(ctx context.Context, serviceName, method, path string, body []byte, headers textproto.MIMEHeader, respObj any) error {
	if headers == nil {
		headers = make(textproto.MIMEHeader)
	}

	if headers.Get(HeaderAccept) == "" {
		headers.Set(HeaderAccept, MediaXml)
	}

	if headers.Get(HeaderContentType) == "" {
		headers.Set(HeaderContentType, MediaXml)
	}

	resp, err := c.Request(ctx, serviceName, method, path, body, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if respObj != nil {
			if err := xml.NewDecoder(resp.Body).Decode(respObj); err != nil {
				return fmt.Errorf("failed to decode XML response: %v", err)
			}
		}
	} else {
		responseBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return NewHttpStatusError(resp.StatusCode, fmt.Sprintf("failed to read response body: %v", readErr))
		}

		return NewHttpStatusError(resp.StatusCode, string(responseBody))
	}

	return nil
}

func (c *Client) Get(ctx context.Context, serviceName string, path string, headers textproto.MIMEHeader) (*http.Response, error) {
	return c.Request(ctx, serviceName, http.MethodGet, path, nil, headers)
}

func (c *Client) Post(ctx context.Context, serviceName string, path string, body []byte, headers textproto.MIMEHeader) (*http.Response, error) {
	return c.Request(ctx, serviceName, http.MethodPost, path, body, headers)
}

func (c *Client) Put(ctx context.Context, serviceName string, path string, body []byte, headers textproto.MIMEHeader) (*http.Response, error) {
	return c.Request(ctx, serviceName, http.MethodPut, path, body, headers)
}

func (c *Client) Delete(ctx context.Context, serviceName string, path string, headers textproto.MIMEHeader) (*http.Response, error) {
	return c.Request(ctx, serviceName, http.MethodDelete, path, nil, headers)
}

func (c *Client) Request(ctx context.Context, serviceName string, method string, path string, body []byte, headers textproto.MIMEHeader) (*http.Response, error) {
	endpoints, err := c.discovery.GetEndpoints(serviceName)
	if err != nil {
		return nil, err
	}

	c.logger.Debug("successfully get endpoints", slog.String(LogKeyService, serviceName), slog.String(LogKeyIps, formatIPs(extractEndpointIPs(endpoints))))

	endpoint, ok := c.getNextEndpoint(serviceName, endpoints)
	if !ok {
		return nil, ErrNoAvailableEndpoint
	}

	c.logger.Debug("choose endpoint", slog.String(LogKeyService, serviceName), slog.String(LogKeyIp, endpoint.Address))

	var prefix string
	if c.tlsConfig != nil {
		prefix = "https://"
	} else {
		prefix = "http://"
	}
	url := fmt.Sprintf("%s%s:%d%s", prefix, endpoint.Address, endpoint.Port, path)

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform HTTP request: %v", err)
	}

	return resp, nil
}

func (c *Client) getNextEndpoint(serviceName string, endpoints []*Endpoint) (*Endpoint, bool) {
	if len(endpoints) == 0 {
		return nil, false
	}

	var newRRIndex atomic.Uint32
	rrIndex, _ := c.rrIndices.LoadOrStore(serviceName, &newRRIndex)

	index := rrIndex.Add(1)

	// index start with 0
	idx := (index - 1) % uint32(len(endpoints))

	return endpoints[int(idx)], true
}
