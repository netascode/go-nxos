// Package nxos is a a Cisco NXOS NX-API REST client library for Go.
package nxos

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// Client is an HTTP NXOS NX-API client.
// Use nxos.NewClient to initiate a client.
// This will ensure proper cookie handling and processing of modifiers.
type Client struct {
	// HttpClient is the *http.Client used for API requests.
	HttpClient *http.Client
	// Url is the NXOS device IP or hostname, e.g. https://10.0.0.1:443 (port is optional).
	Url string
	// Usr is the NXOS device username.
	Usr string
	// Pwd is the NXOS device password.
	Pwd string
	// Insecure determines if insecure https connections are allowed.
	Insecure bool
	// LastRefresh is the timestamp of the last token refresh interval.
	LastRefresh time.Time
	// Token is the current authentication token
	Token string
}

// NewClient creates a new NXOS HTTP client.
// Pass modifiers in to modify the behavior of the client, e.g.
//  client, _ := NewClient("apic", "user", "password", true, RequestTimeout(120))
func NewClient(url, usr, pwd string, insecure bool, mods ...func(*Client)) (Client, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}

	cookieJar, _ := cookiejar.New(nil)
	httpClient := http.Client{
		Timeout:   60 * time.Second,
		Transport: tr,
		Jar:       cookieJar,
	}

	client := Client{
		HttpClient: &httpClient,
		Url:        url,
		Usr:        usr,
		Pwd:        pwd,
		Insecure:   insecure,
	}
	for _, mod := range mods {
		mod(&client)
	}
	return client, nil
}

// NewReq creates a new Req request for this client.
func (client Client) NewReq(method, uri string, body io.Reader, mods ...func(*Req)) Req {
	httpReq, _ := http.NewRequest(method, client.Url+uri+".json", body)
	req := Req{
		HttpReq: httpReq,
		Refresh: true,
	}
	for _, mod := range mods {
		mod(&req)
	}
	return req
}

// RequestTimeout modifies the HTTP request timeout from the default of 60 seconds.
func RequestTimeout(x time.Duration) func(*Client) {
	return func(client *Client) {
		client.HttpClient.Timeout = x * time.Second
	}
}

// Do makes a request.
// Requests for Do are built ouside of the client, e.g.
//
//  req := client.NewReq("GET", "/api/mo/sys/bgp", nil)
//  res, _ := client.Do(req)
func (client *Client) Do(req Req) (Res, error) {
	log.Printf("[DEBUG] HTTP Request: %s, %s, %s", req.HttpReq.Method, req.HttpReq.URL, req.HttpReq.Body)
	httpRes, err := client.HttpClient.Do(req.HttpReq)
	if err != nil {
		return Res{}, err
	}
	defer httpRes.Body.Close()
	body, err := ioutil.ReadAll(httpRes.Body)
	if err != nil {
		return Res{}, errors.New("cannot decode response body")
	}
	res := Res(gjson.ParseBytes(body))
	log.Printf("[DEBUG] HTTP Response: %s", body)
	if httpRes.StatusCode != http.StatusOK {
		return res, fmt.Errorf("received HTTP status %d", httpRes.StatusCode)
	}
	errCode := res.Get("imdata.0.error.attributes.code").Str
	if errCode != "" {
		return res, errors.New("JSON error")
	}
	return res, nil
}

// Get makes a GET request and returns a GJSON result.
// Results will be the raw data structure as returned by the NXOS device, wrapped in imdata, e.g.
//
//  {
//    "totalCount": "1",
//    "imdata": [
//      {
//        "bgpEntity": {
//          "attributes": {
//            "adminSt": "enabled",
//            "dn": "sys/bgp",
//            "name": "bgp"
//          }
//        }
//      }
//    ]
//  }
func (client *Client) Get(path string, mods ...func(*Req)) (Res, error) {
	client.Authenticate()
	req := client.NewReq("GET", path, nil, mods...)
	return client.Do(req)
}

// GetClass makes a GET request by class and unwraps the results.
// Result is removed from imdata, but still wrapped in Class.attributes, e.g.
//  [
//    {
//      "bgpEntity": {
//        "attributes": {
//          "dn": "sys/bgp",
//          "name": "bgp",
//        }
//      }
//    }
//  ]
func (client *Client) GetClass(class string, mods ...func(*Req)) (Res, error) {
	client.Authenticate()
	res, err := client.Get(fmt.Sprintf("/api/class/%s", class), mods...)
	if err != nil {
		return res, err
	}
	return res.Get("imdata"), nil
}

// GetDn makes a GET request by DN.
// Result is removed from imdata and first result is removed from the list, e.g.
//  {
//    "bgpEntity": {
//      "attributes": {
//        "dn": "sys/bgp",
//        "name": "bgp",
//      }
//    }
//  }
func (client *Client) GetDn(dn string, mods ...func(*Req)) (Res, error) {
	client.Authenticate()
	res, err := client.Get(fmt.Sprintf("/api/mo/%s", dn), mods...)
	if err != nil {
		return res, err
	}
	return res.Get("imdata.0"), nil
}

// DeleteDn makes a DELETE request by DN.
func (client *Client) DeleteDn(dn string, mods ...func(*Req)) (Res, error) {
	client.Authenticate()
	req := client.NewReq("DELETE", fmt.Sprintf("/api/mo/%s", dn), nil, mods...)
	return client.Do(req)
}

// Post makes a POST request and returns a GJSON result.
// Hint: Use the Body struct to easily create POST body data.
func (client *Client) Post(dn, data string, mods ...func(*Req)) (Res, error) {
	client.Authenticate()
	req := client.NewReq("POST", fmt.Sprintf("/api/mo/%s", dn), strings.NewReader(data), mods...)
	return client.Do(req)
}

// Put makes a PUT request and returns a GJSON result.
// Hint: Use the Body struct to easily create PUT body data.
func (client *Client) Put(dn, data string, mods ...func(*Req)) (Res, error) {
	client.Authenticate()
	req := client.NewReq("PUT", fmt.Sprintf("/api/mo/%s", dn), strings.NewReader(data), mods...)
	return client.Do(req)
}

// Login authenticates to the NXOS device.
func (client *Client) Login() error {
	data := fmt.Sprintf(`{"aaaUser":{"attributes":{"name":"%s","pwd":"%s"}}}`,
		client.Usr,
		client.Pwd,
	)
	req := client.NewReq("POST", "/api/aaaLogin", strings.NewReader(data), NoRefresh)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	client.Token = res.Get("imdata.0.aaaLogin.attributes.token").Str
	client.LastRefresh = time.Now()
	return nil
}

// Refresh refreshes the authentication token.
// Note that this will be handled automatically be default.
// Refresh will be checked every request and the token will be refreshed after 8 minutes.
// Pass nxos.NoRefresh to prevent automatic refresh handling and handle it directly instead.
func (client *Client) Refresh() error {
	res, err := client.Get("/api/aaaRefresh", NoRefresh)
	if err != nil {
		return err
	}
	client.Token = res.Get("imdata.0.aaaRefresh.attributes.token").Str
	client.LastRefresh = time.Now()
	return nil
}

// Login if no token available or refresh the token if older than 480 seconds.
func (client *Client) Authenticate() error {
	if client.Token == "" {
		return client.Login()
	} else if time.Now().Sub(client.LastRefresh) > 480*time.Second {
		return client.Refresh()
	} else {
		return nil
	}
}
