package nxos

import (
	"net/http"
	neturl "net/url"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Body wraps SJSON for building JSON body strings.
// Usage example:
//
//	Body{}.Set("bgpInst.attributes.asn", "100").Str
type Body struct {
	Str string
}

// Set sets a JSON path to a value.
func (body Body) Set(path, value string) Body {
	res, _ := sjson.Set(body.Str, path, value)
	body.Str = res
	return body
}

// SetRaw sets a JSON path to a raw string value.
// This is primarily used for building up nested structures, e.g.:
//
//	Body{}.SetRaw("bgpInst.attributes", Body{}.Set("asn", "100").Str).Str
func (body Body) SetRaw(path, rawValue string) Body {
	res, _ := sjson.SetRaw(body.Str, path, rawValue)
	body.Str = res
	return body
}

// Delete deletes a JSON path.
func (body Body) Delete(path string) Body {
	res, _ := sjson.Delete(body.Str, path)
	body.Str = res
	return body
}

// Res creates a Res object, i.e. a GJSON result object.
func (body Body) Res() Res {
	return gjson.Parse(body.Str)
}

// Req wraps http.Request for API requests.
type Req struct {
	// HttpReq is the *http.Request obejct.
	HttpReq *http.Request
	// Refresh indicates whether token refresh should be checked for this request.
	// Pass NoRefresh to disable Refresh check.
	Refresh bool
	// LogPayload indicates whether logging of payloads should be enabled.
	LogPayload bool
	// OverrideUrl indicates a URL to use instead
	OverrideUrl string
}

// NoRefresh prevents token refresh check.
// Primarily used by the Login and Refresh methods where this would be redundant.
func NoRefresh(req *Req) {
	req.Refresh = false
}

// NoLogPayload prevents logging of payloads.
// Primarily used by the Login and Refresh methods where this could expose secrets.
func NoLogPayload(req *Req) {
	req.LogPayload = false
}

// Query sets an HTTP query parameter.
//
//	client.GetClass("bgpInst", nxos.Query("query-target-filter", `eq(bgpInst.asn,"100")`))
//
// Or set multiple parameters:
//
//	client.GetClass("bgpInst",
//	  nxos.Query("rsp-subtree-include", "faults"),
//	  nxos.Query("query-target-filter", `eq(bgpInst.asn,"100")`))
func Query(k, v string) func(req *Req) {
	return func(req *Req) {
		q := req.HttpReq.URL.Query()
		q.Add(k, v)
		req.HttpReq.URL.RawQuery = q.Encode()
	}
}

// OverrideUrl uses another url instead of the one when the client was created
func OverrideUrl(url string) func(req *Req) {
	return func(req *Req) {
		oldUrl := req.HttpReq.URL.String()
		newUrl := ""
		i := 0
		for m := 1; m <= 3; m++ {
			x := strings.Index(oldUrl[i:], "/")
			if x < 0 {
				break
			}
			i += x + 1
			if m == 3 {
				newUrl = oldUrl[(i - 1):]
			}
		}
		req.HttpReq.URL, _ = neturl.Parse(url + newUrl)
	}
}
