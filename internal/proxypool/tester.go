package proxypool

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	xproxy "golang.org/x/net/proxy"
)

type TestResult struct {
	Proxy     string `json:"proxy"`
	OK        bool   `json:"ok"`
	IP        string `json:"ip,omitempty"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

func Test(ctx context.Context, proxyURL string, timeout time.Duration) TestResult {
	result := TestResult{Proxy: strings.TrimSpace(proxyURL)}
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	client, err := clientForProxy(result.Proxy, timeout)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org?format=json", nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	resp, err := client.Do(req)
	result.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = fmt.Sprintf("ip check http %d", resp.StatusCode)
		return result
	}
	var payload struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		result.Error = err.Error()
		return result
	}
	result.IP = strings.TrimSpace(payload.IP)
	result.OK = result.IP != ""
	if !result.OK {
		result.Error = "empty ip response"
		return result
	}
	return result
}

func clientForProxy(candidate string, timeout time.Duration) (*http.Client, error) {
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if candidate == "" {
		return &http.Client{Timeout: timeout, Transport: transport}, nil
	}
	parsed, err := url.Parse(candidate)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid proxy url")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		transport.Proxy = http.ProxyURL(parsed)
	case "socks5", "socks5h":
		dialer, err := socksDialer(parsed)
		if err != nil {
			return nil, err
		}
		transport.Proxy = nil
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			type contextDialer interface {
				DialContext(context.Context, string, string) (net.Conn, error)
			}
			if d, ok := dialer.(contextDialer); ok {
				return d.DialContext(ctx, network, address)
			}
			return dialer.Dial(network, address)
		}
	default:
		return nil, fmt.Errorf("unsupported proxy scheme %q", parsed.Scheme)
	}
	return &http.Client{Timeout: timeout, Transport: transport}, nil
}

func socksDialer(parsed *url.URL) (xproxy.Dialer, error) {
	var auth *xproxy.Auth
	if parsed.User != nil {
		password, _ := parsed.User.Password()
		auth = &xproxy.Auth{User: parsed.User.Username(), Password: password}
	}
	return xproxy.SOCKS5("tcp", parsed.Host, auth, xproxy.Direct)
}
