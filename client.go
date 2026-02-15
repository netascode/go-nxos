// Package nxos is a a Cisco NXOS NX-API REST client library for Go.
package nxos

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const DefaultMaxRetries int = 3
const DefaultBackoffMinDelay int = 4
const DefaultBackoffMaxDelay int = 60
const DefaultBackoffDelayFactor float64 = 3

// Client is an HTTP NXOS NX-API client.
// Use nxos.NewClient to initiate a client.
// This will ensure proper cookie handling and processing of modifiers.
type Client struct {
	// HttpClient is the *http.Client used for API requests.
	HttpClient *http.Client
	// List of URLs.
	Url string
	// LastRefresh is the timestamp of the last token refresh interval.
	LastRefresh time.Time
	// Token is the current authentication token
	Token string
	// Usr is the NXOS device username.
	Usr string
	// Pwd is the NXOS device password.
	Pwd string
	// Insecure determines if insecure https connections are allowed.
	Insecure bool
	// Maximum number of retries
	MaxRetries int
	// Minimum delay between two retries
	BackoffMinDelay int
	// Maximum delay between two retries
	BackoffMaxDelay int
	// Backoff delay factor
	BackoffDelayFactor float64
	// Mutex for authentication token refresh
	authMutex sync.Mutex
}

// NewClient creates a new NXOS HTTP client.
// Pass modifiers in to modify the behavior of the client, e.g.
//
//	client, _ := NewClient("apic", "user", "password", true, RequestTimeout(120))
func NewClient(url, usr, pwd string, insecure bool, mods ...func(*Client)) (Client, error) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: insecure,
		ClientSessionCache: tls.NewLRUClientSessionCache(0),
	}
	tr.MaxIdleConnsPerHost = 32

	cookieJar, _ := cookiejar.New(nil)
	httpClient := http.Client{
		Timeout:   60 * time.Second,
		Transport: tr,
		Jar:       cookieJar,
	}

	client := Client{
		HttpClient:         &httpClient,
		Url:                url,
		Usr:                usr,
		Pwd:                pwd,
		Insecure:           insecure,
		MaxRetries:         DefaultMaxRetries,
		BackoffMinDelay:    DefaultBackoffMinDelay,
		BackoffMaxDelay:    DefaultBackoffMaxDelay,
		BackoffDelayFactor: DefaultBackoffDelayFactor,
	}

	for _, mod := range mods {
		mod(&client)
	}
	return client, nil
}

// RequestTimeout modifies the HTTP request timeout from the default of 60 seconds.
func RequestTimeout(x time.Duration) func(*Client) {
	return func(client *Client) {
		client.HttpClient.Timeout = x * time.Second
	}
}

// MaxRetries modifies the maximum number of retries from the default of 3.
func MaxRetries(x int) func(*Client) {
	return func(client *Client) {
		client.MaxRetries = x
	}
}

// BackoffMinDelay modifies the minimum delay between two retries from the default of 4.
func BackoffMinDelay(x int) func(*Client) {
	return func(client *Client) {
		client.BackoffMinDelay = x
	}
}

// BackoffMaxDelay modifies the maximum delay between two retries from the default of 60.
func BackoffMaxDelay(x int) func(*Client) {
	return func(client *Client) {
		client.BackoffMaxDelay = x
	}
}

// BackoffDelayFactor modifies the backoff delay factor from the default of 3.
func BackoffDelayFactor(x float64) func(*Client) {
	return func(client *Client) {
		client.BackoffDelayFactor = x
	}
}

// NewReq creates a new Req request for this client.
func (client Client) NewReq(method, uri string, body io.Reader, mods ...func(*Req)) Req {
	httpReq, _ := http.NewRequest(method, client.Url+uri+".json", body)
	req := Req{
		HttpReq:    httpReq,
		Refresh:    true,
		LogPayload: true,
	}
	for _, mod := range mods {
		mod(&req)
	}
	return req
}

// Do makes a request.
// Requests for Do are built ouside of the client, e.g.
//
//	req := client.NewReq("GET", "/api/mo/sys/bgp", nil)
//	res, _ := client.Do(req)
func (client *Client) Do(req Req) (Res, error) {
	// retain the request body across multiple attempts
	var body []byte
	if req.HttpReq.Body != nil {
		body, _ = ioutil.ReadAll(req.HttpReq.Body)
	}

	var res Res

	for attempts := 0; ; attempts++ {
		req.HttpReq.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		if req.LogPayload {
			log.Printf("[DEBUG] HTTP Request: %s, %s, %s", req.HttpReq.Method, req.HttpReq.URL, req.HttpReq.Body)
		} else {
			log.Printf("[DEBUG] HTTP Request: %s, %s", req.HttpReq.Method, req.HttpReq.URL)
		}

		httpRes, err := client.HttpClient.Do(req.HttpReq)
		if err != nil {
			if ok := client.Backoff(attempts); !ok {
				log.Printf("[ERROR] HTTP Connection error occured: %+v", err)
				log.Printf("[DEBUG] Exit from Do method")
				return Res{}, err
			} else {
				log.Printf("[ERROR] HTTP Connection failed: %s, retries: %v", err, attempts)
				continue
			}
		}

		bodyBytes, err := ioutil.ReadAll(httpRes.Body)
		httpRes.Body.Close()
		if err != nil {
			if ok := client.Backoff(attempts); !ok {
				log.Printf("[ERROR] Cannot decode response body: %+v", err)
				log.Printf("[DEBUG] Exit from Do method")
				return Res{}, err
			} else {
				log.Printf("[ERROR] Cannot decode response body: %s, retries: %v", err, attempts)
				continue
			}
		}
		res = Res(gjson.ParseBytes(bodyBytes))
		if req.LogPayload {
			log.Printf("[DEBUG] HTTP Response: %s", res.Raw)
		}

		if (httpRes.StatusCode < 500 || httpRes.StatusCode > 504) && httpRes.StatusCode != 405 {
			log.Printf("[DEBUG] Exit from Do method")
			break
		} else {
			if ok := client.Backoff(attempts); !ok {
				log.Printf("[ERROR] HTTP Request failed: StatusCode %v", httpRes.StatusCode)
				log.Printf("[DEBUG] Exit from Do method")
				return Res{}, fmt.Errorf("HTTP Request failed: StatusCode %v", httpRes.StatusCode)
			} else {
				log.Printf("[ERROR] HTTP Request failed: StatusCode %v, Retries: %v", httpRes.StatusCode, attempts)
				continue
			}
		}
	}

	errCode := res.Get("imdata.0.error.attributes.code").Str
	if errCode != "" {
		log.Printf("[ERROR] JSON error: %s", res.Raw)
		return res, fmt.Errorf("JSON error: %s", res.Raw)
	}
	return res, nil
}

// Get makes a GET request and returns a GJSON result.
// Results will be the raw data structure as returned by the NXOS device, wrapped in imdata, e.g.
//
//	{
//	  "totalCount": "1",
//	  "imdata": [
//	    {
//	      "bgpEntity": {
//	        "attributes": {
//	          "adminSt": "enabled",
//	          "dn": "sys/bgp",
//	          "name": "bgp"
//	        }
//	      }
//	    }
//	  ]
//	}
func (client *Client) Get(path string, mods ...func(*Req)) (Res, error) {
	req := client.NewReq("GET", path, nil, mods...)
	client.Authenticate()
	return client.Do(req)
}

// GetClass makes a GET request by class and unwraps the results.
// Result is removed from imdata, but still wrapped in Class.attributes, e.g.
//
//	[
//	  {
//	    "bgpEntity": {
//	      "attributes": {
//	        "dn": "sys/bgp",
//	        "name": "bgp",
//	      }
//	    }
//	  }
//	]
func (client *Client) GetClass(class string, mods ...func(*Req)) (Res, error) {
	res, err := client.Get(fmt.Sprintf("/api/class/%s", class), mods...)
	if err != nil {
		return res, err
	}
	return res.Get("imdata"), nil
}

// GetDn makes a GET request by DN.
// Result is removed from imdata and first result is removed from the list, e.g.
//
//	{
//	  "bgpEntity": {
//	    "attributes": {
//	      "dn": "sys/bgp",
//	      "name": "bgp",
//	    }
//	  }
//	}
func (client *Client) GetDn(dn string, mods ...func(*Req)) (Res, error) {
	res, err := client.Get(fmt.Sprintf("/api/mo/%s", dn), mods...)
	if err != nil {
		return res, err
	}
	return res.Get("imdata.0"), nil
}

// DeleteDn makes a DELETE request by DN.
func (client *Client) DeleteDn(dn string, mods ...func(*Req)) (Res, error) {
	req := client.NewReq("DELETE", fmt.Sprintf("/api/mo/%s", dn), nil, mods...)
	client.Authenticate()
	return client.Do(req)
}

// Post makes a POST request and returns a GJSON result.
// Hint: Use the Body struct to easily create POST body data.
func (client *Client) Post(dn, data string, mods ...func(*Req)) (Res, error) {
	req := client.NewReq("POST", fmt.Sprintf("/api/mo/%s", dn), strings.NewReader(data), mods...)
	client.Authenticate()
	return client.Do(req)
}

// Put makes a PUT request and returns a GJSON result.
// Hint: Use the Body struct to easily create PUT body data.
func (client *Client) Put(dn, data string, mods ...func(*Req)) (Res, error) {
	req := client.NewReq("PUT", fmt.Sprintf("/api/mo/%s", dn), strings.NewReader(data), mods...)
	client.Authenticate()
	return client.Do(req)
}

// JsonRpc makes a JSON-RPC request and returns a GJSON result.
func (client *Client) JsonRpc(command string, mods ...func(*Req)) (Res, error) {
	data, _ := sjson.Set("[]", "0.jsonrpc", "2.0")
	data, _ = sjson.Set(data, "0.method", "cli")
	data, _ = sjson.Set(data, "0.params.cmd", command)
	data, _ = sjson.Set(data, "0.params.version", 1)
	data, _ = sjson.Set(data, "0.id", 1)
	req := client.NewReq("POST", "/ins", strings.NewReader(data), mods...)
	req.HttpReq.Header.Add("Content-Type", "application/json-rpc")
	req.HttpReq.Header.Add("Cache-Control", "no-cache")
	req.HttpReq.SetBasicAuth(client.Usr, client.Pwd)
	return client.Do(req)
}

// Login authenticates to the NXOS device.
func (client *Client) Login() error {
	data := fmt.Sprintf(`{"aaaUser":{"attributes":{"name":"%s","pwd":"%s"}}}`,
		client.Usr,
		client.Pwd,
	)
	req := client.NewReq("POST", "/api/aaaLogin", strings.NewReader(data), NoRefresh, NoLogPayload)
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
	res, err := client.Get("/api/aaaRefresh", NoRefresh, NoLogPayload)
	if err != nil {
		return err
	}
	client.Token = res.Get("imdata.0.aaaRefresh.attributes.token").Str
	client.LastRefresh = time.Now()
	return nil
}

// Login if no token available or refresh the token if older than 480 seconds.
func (client *Client) Authenticate() error {
	client.authMutex.Lock()
	defer client.authMutex.Unlock()
	if client.Token == "" {
		return client.Login()
	} else if time.Since(client.LastRefresh) > 480*time.Second {
		return client.Refresh()
	} else {
		return nil
	}
}

// Backoff waits following an exponential backoff algorithm
func (client *Client) Backoff(attempts int) bool {
	log.Printf("[DEBUG] Begining backoff method: attempts %v on %v", attempts, client.MaxRetries)
	if attempts >= client.MaxRetries {
		log.Printf("[DEBUG] Exit from backoff method with return value false")
		return false
	}

	minDelay := time.Duration(client.BackoffMinDelay) * time.Second
	maxDelay := time.Duration(client.BackoffMaxDelay) * time.Second

	min := float64(minDelay)
	backoff := min * math.Pow(client.BackoffDelayFactor, float64(attempts))
	if backoff > float64(maxDelay) {
		backoff = float64(maxDelay)
	}
	backoff = (rand.Float64()/2+0.5)*(backoff-min) + min
	backoffDuration := time.Duration(backoff)
	log.Printf("[TRACE] Starting sleeping for %v", backoffDuration.Round(time.Second))
	time.Sleep(backoffDuration)
	log.Printf("[DEBUG] Exit from backoff method with return value true")
	return true
}
