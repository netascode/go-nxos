package nxos

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

const (
	testURL = "https://10.0.0.1"
)

func testClient() Client {
	client, _ := NewClient(testURL, "usr", "pwd", true, MaxRetries(0))
	client.LastRefresh = time.Now()
	gock.InterceptClient(client.HttpClient)
	return client
}

// ErrReader implements the io.Reader interface and fails on Read.
type ErrReader struct{}

// Read mocks failing io.Reader test cases.
func (r ErrReader) Read(buf []byte) (int, error) {
	return 0, errors.New("fail")
}

// TestNewClient tests the NewClient function.
func TestNewClient(t *testing.T) {
	client, _ := NewClient(testURL, "usr", "pwd", true, RequestTimeout(120))
	assert.Equal(t, client.HttpClient.Timeout, 120*time.Second)
}

// TestClientLogin tests the Client::Login method.
func TestClientLogin(t *testing.T) {
	defer gock.Off()
	client := testClient()

	// Successful login
	gock.New(testURL).Post("/api/aaaLogin.json").Reply(200)
	assert.NoError(t, client.Login())

	// Invalid HTTP status code
	gock.New(testURL).Post("/api/aaaLogin.json").Reply(405)
	assert.Error(t, client.Login())

	// JSON error from Client
	gock.New(testURL).
		Post("/api/aaaLogin.json").
		Reply(200).
		BodyString(Body{}.Set("imdata.0.error.attributes.code", "123").Str)
	assert.Error(t, client.Login())
}

// TestClientRefresh tests the Client::Refresh method.
func TestClientRefresh(t *testing.T) {
	defer gock.Off()
	client := testClient()

	gock.New(testURL).Get("/api/aaaRefresh.json").Reply(200)
	assert.NoError(t, client.Refresh())
}

// TestClientAuthenticate tests the Client::Authenticate method.
func TestClientAuthenticate(t *testing.T) {
	defer gock.Off()
	client := testClient()

	// Force token refresh and throw an error
	client.LastRefresh = time.Now().AddDate(0, 0, -1)
	gock.New(testURL).
		Get("/api/aaaRefresh.json").
		ReplyError(errors.New("fail"))
	err := client.Authenticate()
	assert.Error(t, err)
}

// TestClientGet tests the Client::Get method.
func TestClientGet(t *testing.T) {
	defer gock.Off()
	client := testClient()
	var err error

	// Success
	gock.New(testURL).Get("/url.json").Reply(200)
	_, err = client.Get("/url")
	assert.NoError(t, err)

	// HTTP error
	gock.New(testURL).Get("/url.json").ReplyError(errors.New("fail"))
	_, err = client.Get("/url")
	assert.Error(t, err)

	// Invalid HTTP status code
	gock.New(testURL).Get("/url.json").Reply(405)
	_, err = client.Get("/url")
	assert.Error(t, err)

	// Error decoding response body
	gock.New(testURL).
		Get("/url.json").
		Reply(200).
		Map(func(res *http.Response) *http.Response {
			res.Body = ioutil.NopCloser(ErrReader{})
			return res
		})
	_, err = client.Get("/url")
	assert.Error(t, err)
}

// TestClientGetClass tests the Client::GetClass method.
func TestClientGetClass(t *testing.T) {
	defer gock.Off()
	client := testClient()

	// Success
	gock.New(testURL).
		Get("/api/class/l1PhysIf.json").
		Reply(200).
		BodyString(Body{}.
			Set("imdata.0.l1PhysIf.attributes.id", "eth1/1").
			Set("imdata.1.l1PhysIf.attributes.id", "eth1/2").
			Str)
	res, _ := client.GetClass("l1PhysIf")
	if !assert.Len(t, res.Array(), 2) {
		fmt.Println(res.Get("@pretty"))
	}
	if !assert.Equal(t, "eth1/2", res.Get("1.l1PhysIf.attributes.id").Str) {
		fmt.Println(res.Get("@pretty"))
	}

	// HTTP error
	gock.New(testURL).Get("/api/class/test.json").ReplyError(errors.New("fail"))
	_, err := client.GetClass("test")
	assert.Error(t, err)
}

// TestClientGetDn tests the Client::GetDn method.
func TestClientGetDn(t *testing.T) {
	defer gock.Off()
	client := testClient()

	// Success
	gock.New(testURL).
		Get("/api/mo/sys/intf/phys-[eth1/1].json").
		Reply(200).
		BodyString(Body{}.Set("imdata.0.l1PhysIf.attributes.id", "eth1/1").Str)
	res, _ := client.GetDn("sys/intf/phys-[eth1/1]")
	if !assert.Equal(t, "eth1/1", res.Get("l1PhysIf.attributes.id").Str) {
		fmt.Println(res.Get("@pretty"))
	}

	// HTTP error
	gock.New(testURL).
		Get("/api/mo/uni/fail.json").
		ReplyError(errors.New("fail"))
	_, err := client.GetDn("uni/fail")
	assert.Error(t, err)
}

// TestClientDeleteDn tests the Client::DeleteDn method.
func TestClientDeleteDn(t *testing.T) {
	defer gock.Off()
	client := testClient()

	// Success
	gock.New(testURL).
		Delete("/api/mo/sys/intf/phys-[eth1/1].json").
		Reply(200)
	_, err := client.DeleteDn("sys/intf/phys-[eth1/1]")
	assert.NoError(t, err)

	// HTTP error
	gock.New(testURL).
		Delete("/api/mo/uni/fail.json").
		ReplyError(errors.New("fail"))
	_, err = client.DeleteDn("uni/fail")
	assert.Error(t, err)
}

// TestClientPost tests the Client::Post method.
func TestClientPost(t *testing.T) {
	defer gock.Off()
	client := testClient()

	var err error

	// Success
	gock.New(testURL).Post("/url.json").Reply(200)
	_, err = client.Post("/url", "{}")
	assert.NoError(t, err)

	// HTTP error
	gock.New(testURL).Post("/url.json").ReplyError(errors.New("fail"))
	_, err = client.Post("/url", "{}")
	assert.Error(t, err)

	// Invalid HTTP status code
	gock.New(testURL).Post("/url.json").Reply(405)
	_, err = client.Post("/url", "{}")
	assert.Error(t, err)

	// Error decoding response body
	gock.New(testURL).
		Post("/url.json").
		Reply(200).
		Map(func(res *http.Response) *http.Response {
			res.Body = ioutil.NopCloser(ErrReader{})
			return res
		})
	_, err = client.Post("/url", "{}")
	assert.Error(t, err)
}

// TestClientPost tests the Client::Post method.
func TestClientPut(t *testing.T) {
	defer gock.Off()
	client := testClient()

	var err error

	// Success
	gock.New(testURL).Put("/url.json").Reply(200)
	_, err = client.Put("/url", "{}")
	assert.NoError(t, err)

	// HTTP error
	gock.New(testURL).Put("/url.json").ReplyError(errors.New("fail"))
	_, err = client.Put("/url", "{}")
	assert.Error(t, err)

	// Invalid HTTP status code
	gock.New(testURL).Put("/url.json").Reply(405)
	_, err = client.Put("/url", "{}")
	assert.Error(t, err)

	// Error decoding response body
	gock.New(testURL).
		Put("/url.json").
		Reply(200).
		Map(func(res *http.Response) *http.Response {
			res.Body = ioutil.NopCloser(ErrReader{})
			return res
		})
	_, err = client.Put("/url", "{}")
	assert.Error(t, err)
}

// TestClientJsonRpc tests the Client::JsonRpc method.
func TestClientJsonRpc(t *testing.T) {
	defer gock.Off()
	client := testClient()

	var err error

	// Success
	gock.New(testURL).Post("/ins").Reply(200)
	_, err = client.JsonRpc("copy run start")
	assert.NoError(t, err)
}
