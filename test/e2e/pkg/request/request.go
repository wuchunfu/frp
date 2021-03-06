package request

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	libnet "github.com/fatedier/golib/net"
)

type Request struct {
	protocol string

	// for all protocol
	addr    string
	port    int
	body    []byte
	timeout time.Duration

	// for http
	method  string
	host    string
	path    string
	headers map[string]string

	proxyURL string
}

func New() *Request {
	return &Request{
		protocol: "tcp",
		addr:     "127.0.0.1",

		method: "GET",
		path:   "/",
	}
}

func (r *Request) Protocol(protocol string) *Request {
	r.protocol = protocol
	return r
}

func (r *Request) TCP() *Request {
	r.protocol = "tcp"
	return r
}

func (r *Request) UDP() *Request {
	r.protocol = "udp"
	return r
}

func (r *Request) HTTP() *Request {
	r.protocol = "http"
	return r
}

func (r *Request) Proxy(url string) *Request {
	r.proxyURL = url
	return r
}

func (r *Request) Addr(addr string) *Request {
	r.addr = addr
	return r
}

func (r *Request) Port(port int) *Request {
	r.port = port
	return r
}

func (r *Request) HTTPParams(method, host, path string, headers map[string]string) *Request {
	r.method = method
	r.host = host
	r.path = path
	r.headers = headers
	return r
}

func (r *Request) HTTPHost(host string) *Request {
	r.host = host
	return r
}

func (r *Request) HTTPPath(path string) *Request {
	r.path = path
	return r
}

func (r *Request) HTTPHeaders(headers map[string]string) *Request {
	r.headers = headers
	return r
}

func (r *Request) Timeout(timeout time.Duration) *Request {
	r.timeout = timeout
	return r
}

func (r *Request) Body(content []byte) *Request {
	r.body = content
	return r
}

func (r *Request) Do() (*Response, error) {
	var (
		conn net.Conn
		err  error
	)

	addr := net.JoinHostPort(r.addr, strconv.Itoa(r.port))
	// for protocol http
	if r.protocol == "http" {
		return sendHTTPRequest(r.method, fmt.Sprintf("http://%s%s", addr, r.path),
			r.host, r.headers, r.proxyURL, r.body)
	}

	// for protocol tcp and udp
	if len(r.proxyURL) > 0 {
		if r.protocol != "tcp" {
			return nil, fmt.Errorf("only tcp protocol is allowed for proxy")
		}
		conn, err = libnet.DialTcpByProxy(r.proxyURL, addr)
		if err != nil {
			return nil, err
		}
	} else {
		switch r.protocol {
		case "tcp":
			conn, err = net.Dial("tcp", addr)
		case "udp":
			conn, err = net.Dial("udp", addr)
		default:
			return nil, fmt.Errorf("invalid protocol")
		}
		if err != nil {
			return nil, err
		}
	}

	defer conn.Close()
	if r.timeout > 0 {
		conn.SetDeadline(time.Now().Add(r.timeout))
	}
	buf, err := sendRequestByConn(conn, r.body)
	if err != nil {
		return nil, err
	}
	return &Response{Content: buf}, nil
}

type Response struct {
	Code    int
	Header  http.Header
	Content []byte
}

func sendHTTPRequest(method, urlstr string, host string, headers map[string]string, proxy string, body []byte) (*Response, error) {
	var inBody io.Reader
	if len(body) != 0 {
		inBody = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, urlstr, inBody)
	if err != nil {
		return nil, err
	}
	if host != "" {
		req.Host = host
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	tr := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	if len(proxy) != 0 {
		tr.Proxy = func(req *http.Request) (*url.URL, error) {
			return url.Parse(proxy)
		}
	}
	client := http.Client{Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	ret := &Response{Code: resp.StatusCode, Header: resp.Header}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	ret.Content = buf
	return ret, nil
}

func sendRequestByConn(c net.Conn, content []byte) ([]byte, error) {
	_, err := c.Write(content)
	if err != nil {
		return nil, fmt.Errorf("write error: %v", err)
	}

	buf := make([]byte, 2048)
	n, err := c.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("read error: %v", err)
	}
	return buf[:n], nil
}
