# go-nxos

`go-nxos` is a Go client library for Cisco NX-OS devices. It is based on Nathan's excellent [goaci](https://github.com/brightpuddle/goaci) module and features a simple, extensible API and [advanced JSON manipulation](#result-manipulation).

## Getting Started

### Installing

To start using `go-nxos`, install Go and `go get`:

`$ go get -u github.com/netascode/go-nxos`

### Basic Usage

```go
package main

import "github.com/netascode/go-nxos"

func main() {
    client, _ := nxos.NewClient("1.1.1.1", "user", "pwd", true)

    res, _ := client.Get("/api/mo/sys/intf/phys-[eth1/1]")
    println(res.Get("imdata.0.*.attributes.id").String())
}
```

This will print:
```
eth1/1
```

#### Result manipulation

`nxos.Result` uses GJSON to simplify handling JSON results. See the [GJSON](https://github.com/tidwall/gjson) documentation for more detail.

```go
res, _ := client.GetClass("l1PhysIf")
println(res.Get("0.l1PhysIf.attributes.name").String()) // name of first physical interface

for _, int := range res.Array() {
    println(int.Get("*.attributes|@pretty")) // pretty print physical interface attributes
}

for _, attr := range res.Get("#.l1PhysIf.attributes").Array() {
    println(attr.Get("@pretty")) // pretty print BD attributes
}
```

#### Helpers for common patterns

```go
res, _ := client.GetDn("sys/intf/phys-[eth1/1]")
res, _ := client.GetClass("l1PhysIf")
res, _ := client.DeleteDn("sys/userext/user-[testuser]")
```

#### Query parameters

Pass the `nxos.Query` object to the `Get` request to add query parameters:

```go
queryInfra := nxos.Query("query-target-filter", `eq(l1PhysIf.id,"eth1/1")`)
res, _ := client.GetClass("l1PhysIf", queryInfra)
```

Pass as many parameters as needed:

```go
res, _ := client.GetClass("interfaceEntity",
    nxos.Query("rsp-subtree-include", "l1PhysIf"),
    nxos.Query("query-target-filter", `eq(l1PhysIf.id,"eth1/1")`)
)
```

#### POST data creation

`nxos.Body` is a wrapper for [SJSON](https://github.com/tidwall/sjson). SJSON supports a path syntax simplifying JSON creation.

```go
exampleInt := nxos.Body{}.Set("l1PhysIf.attributes.id", "eth1/1").Str
client.Post("/api/mo/sys/intf/phys-[eth1/1]", exampleInt)
```

These can be chained:

```go
int1 := nxos.Body{}.
    Set("l1PhysIf.attributes.id", "eth1/1").
    Set("l1PhysIf.attributes.mode", "trunk")
```

...or nested:

```go
attrs := nxos.Body{}.
    Set("id", "eth1/1").
    Set("mode", "trunk").
    Str
int1 := nxos.Body{}.SetRaw("l1PhysIf.attributes", attrs).Str
```

#### Token refresh

Token refresh is handled automatically. The client keeps a timer and checks elapsed time on each request, refreshing the token every 8 minutes. This can be handled manually if desired:

```go
res, _ := client.Get("/api/...", nxos.NoRefresh)
client.Refresh()
```

## Documentation

See the [documentation](https://godoc.org/github.com/netascode/go-nxos) for more details.
