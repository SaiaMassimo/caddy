// Copyright 2015 Matthew Holt and The Caddy Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reverseproxy

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	weakrand "math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
	"go.uber.org/zap"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyevents"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	memento "github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy/memento"
)

func init() {
	caddy.RegisterModule(RandomSelection{})
	caddy.RegisterModule(RandomChoiceSelection{})
	caddy.RegisterModule(LeastConnSelection{})
	caddy.RegisterModule(RoundRobinSelection{})
	caddy.RegisterModule(WeightedRoundRobinSelection{})
	caddy.RegisterModule(FirstSelection{})
	caddy.RegisterModule(IPHashSelection{})
	caddy.RegisterModule(ClientIPHashSelection{})
	caddy.RegisterModule(URIHashSelection{})
	caddy.RegisterModule(QueryHashSelection{})
	caddy.RegisterModule(HeaderHashSelection{})
	caddy.RegisterModule(CookieHashSelection{})
	caddy.RegisterModule(MementoSelection{})
	caddy.RegisterModule(WeightedMementoSelection{})
}

// RandomSelection is a policy that selects
// an available host at random.
type RandomSelection struct{}

// CaddyModule returns the Caddy module information.
func (RandomSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.random",
		New: func() caddy.Module { return new(RandomSelection) },
	}
}

// Select returns an available host, if any.
func (r RandomSelection) Select(pool UpstreamPool, request *http.Request, _ http.ResponseWriter) *Upstream {
	return selectRandomHost(pool)
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (r *RandomSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// WeightedRoundRobinSelection is a policy that selects
// a host based on weighted round-robin ordering.
type WeightedRoundRobinSelection struct {
	// The weight of each upstream in order,
	// corresponding with the list of upstreams configured.
	Weights     []int `json:"weights,omitempty"`
	index       uint32
	totalWeight int
}

// CaddyModule returns the Caddy module information.
func (WeightedRoundRobinSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "http.reverse_proxy.selection_policies.weighted_round_robin",
		New: func() caddy.Module {
			return new(WeightedRoundRobinSelection)
		},
	}
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (r *WeightedRoundRobinSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume policy name

	args := d.RemainingArgs()
	if len(args) == 0 {
		return d.ArgErr()
	}

	for _, weight := range args {
		weightInt, err := strconv.Atoi(weight)
		if err != nil {
			return d.Errf("invalid weight value '%s': %v", weight, err)
		}
		if weightInt < 0 {
			return d.Errf("invalid weight value '%s': weight should be non-negative", weight)
		}
		r.Weights = append(r.Weights, weightInt)
	}
	return nil
}

// Provision sets up r.
func (r *WeightedRoundRobinSelection) Provision(ctx caddy.Context) error {
	for _, weight := range r.Weights {
		r.totalWeight += weight
	}
	return nil
}

// Select returns an available host, if any.
func (r *WeightedRoundRobinSelection) Select(pool UpstreamPool, _ *http.Request, _ http.ResponseWriter) *Upstream {
	if len(pool) == 0 {
		return nil
	}
	if len(r.Weights) < 2 {
		return pool[0]
	}
	var index, totalWeight int
	var weights []int

	for _, w := range r.Weights {
		if w > 0 {
			weights = append(weights, w)
		}
	}
	currentWeight := int(atomic.AddUint32(&r.index, 1)) % r.totalWeight
	for i, weight := range weights {
		totalWeight += weight
		if currentWeight < totalWeight {
			index = i
			break
		}
	}

	upstreams := make([]*Upstream, 0, len(weights))
	for i, upstream := range pool {
		if !upstream.Available() || r.Weights[i] == 0 {
			continue
		}
		upstreams = append(upstreams, upstream)
		if len(upstreams) == cap(upstreams) {
			break
		}
	}
	if len(upstreams) == 0 {
		return nil
	}
	return upstreams[index%len(upstreams)]
}

// RandomChoiceSelection is a policy that selects
// two or more available hosts at random, then
// chooses the one with the least load.
type RandomChoiceSelection struct {
	// The size of the sub-pool created from the larger upstream pool. The default value
	// is 2 and the maximum at selection time is the size of the upstream pool.
	Choose int `json:"choose,omitempty"`
}

// CaddyModule returns the Caddy module information.
func (RandomChoiceSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.random_choose",
		New: func() caddy.Module { return new(RandomChoiceSelection) },
	}
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (r *RandomChoiceSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume policy name

	if !d.NextArg() {
		return d.ArgErr()
	}
	chooseStr := d.Val()
	choose, err := strconv.Atoi(chooseStr)
	if err != nil {
		return d.Errf("invalid choice value '%s': %v", chooseStr, err)
	}
	r.Choose = choose
	return nil
}

// Provision sets up r.
func (r *RandomChoiceSelection) Provision(ctx caddy.Context) error {
	if r.Choose == 0 {
		r.Choose = 2
	}
	return nil
}

// Validate ensures that r's configuration is valid.
func (r RandomChoiceSelection) Validate() error {
	if r.Choose < 2 {
		return fmt.Errorf("choose must be at least 2")
	}
	return nil
}

// Select returns an available host, if any.
func (r RandomChoiceSelection) Select(pool UpstreamPool, _ *http.Request, _ http.ResponseWriter) *Upstream {
	k := min(r.Choose, len(pool))
	choices := make([]*Upstream, k)
	for i, upstream := range pool {
		if !upstream.Available() {
			continue
		}
		j := weakrand.Intn(i + 1) //nolint:gosec
		if j < k {
			choices[j] = upstream
		}
	}
	return leastRequests(choices)
}

// LeastConnSelection is a policy that selects the
// host with the least active requests. If multiple
// hosts have the same fewest number, one is chosen
// randomly. The term "conn" or "connection" is used
// in this policy name due to its similar meaning in
// other software, but our load balancer actually
// counts active requests rather than connections,
// since these days requests are multiplexed onto
// shared connections.
type LeastConnSelection struct{}

// CaddyModule returns the Caddy module information.
func (LeastConnSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.least_conn",
		New: func() caddy.Module { return new(LeastConnSelection) },
	}
}

// Select selects the up host with the least number of connections in the
// pool. If more than one host has the same least number of connections,
// one of the hosts is chosen at random.
func (LeastConnSelection) Select(pool UpstreamPool, _ *http.Request, _ http.ResponseWriter) *Upstream {
	var bestHost *Upstream
	var count int
	leastReqs := -1

	for _, host := range pool {
		if !host.Available() {
			continue
		}
		numReqs := host.NumRequests()
		if leastReqs == -1 || numReqs < leastReqs {
			leastReqs = numReqs
			count = 0
		}

		// among hosts with same least connections, perform a reservoir
		// sample: https://en.wikipedia.org/wiki/Reservoir_sampling
		if numReqs == leastReqs {
			count++
			if count == 1 || (weakrand.Int()%count) == 0 { //nolint:gosec
				bestHost = host
			}
		}
	}

	return bestHost
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (r *LeastConnSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// RoundRobinSelection is a policy that selects
// a host based on round-robin ordering.
type RoundRobinSelection struct {
	robin uint32
}

// CaddyModule returns the Caddy module information.
func (RoundRobinSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.round_robin",
		New: func() caddy.Module { return new(RoundRobinSelection) },
	}
}

// Select returns an available host, if any.
func (r *RoundRobinSelection) Select(pool UpstreamPool, _ *http.Request, _ http.ResponseWriter) *Upstream {
	n := uint32(len(pool))
	if n == 0 {
		return nil
	}
	for i := uint32(0); i < n; i++ {
		robin := atomic.AddUint32(&r.robin, 1)
		host := pool[robin%n]
		if host.Available() {
			return host
		}
	}
	return nil
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (r *RoundRobinSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// FirstSelection is a policy that selects
// the first available host.
type FirstSelection struct{}

// CaddyModule returns the Caddy module information.
func (FirstSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.first",
		New: func() caddy.Module { return new(FirstSelection) },
	}
}

// Select returns an available host, if any.
func (FirstSelection) Select(pool UpstreamPool, _ *http.Request, _ http.ResponseWriter) *Upstream {
	for _, host := range pool {
		if host.Available() {
			return host
		}
	}
	return nil
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (r *FirstSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// IPHashSelection is a policy that selects a host
// based on hashing the remote IP of the request.
type IPHashSelection struct{}

// CaddyModule returns the Caddy module information.
func (IPHashSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.ip_hash",
		New: func() caddy.Module { return new(IPHashSelection) },
	}
}

// Select returns an available host, if any.
func (IPHashSelection) Select(pool UpstreamPool, req *http.Request, _ http.ResponseWriter) *Upstream {
	clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		clientIP = req.RemoteAddr
	}
	return hostByHashing(pool, clientIP)
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (r *IPHashSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// ClientIPHashSelection is a policy that selects a host
// based on hashing the client IP of the request, as determined
// by the HTTP app's trusted proxies settings.
type ClientIPHashSelection struct{}

// CaddyModule returns the Caddy module information.
func (ClientIPHashSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.client_ip_hash",
		New: func() caddy.Module { return new(ClientIPHashSelection) },
	}
}

// Select returns an available host, if any.
func (ClientIPHashSelection) Select(pool UpstreamPool, req *http.Request, _ http.ResponseWriter) *Upstream {
	address := caddyhttp.GetVar(req.Context(), caddyhttp.ClientIPVarKey).(string)
	clientIP, _, err := net.SplitHostPort(address)
	if err != nil {
		clientIP = address // no port
	}
	return hostByHashing(pool, clientIP)
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (r *ClientIPHashSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// URIHashSelection is a policy that selects a
// host by hashing the request URI.
type URIHashSelection struct{}

// CaddyModule returns the Caddy module information.
func (URIHashSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.uri_hash",
		New: func() caddy.Module { return new(URIHashSelection) },
	}
}

// Select returns an available host, if any.
func (URIHashSelection) Select(pool UpstreamPool, req *http.Request, _ http.ResponseWriter) *Upstream {
	return hostByHashing(pool, req.RequestURI)
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (r *URIHashSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume policy name
	if d.NextArg() {
		return d.ArgErr()
	}
	return nil
}

// QueryHashSelection is a policy that selects
// a host based on a given request query parameter.
type QueryHashSelection struct {
	// The query key whose value is to be hashed and used for upstream selection.
	Key string `json:"key,omitempty"`

	// The fallback policy to use if the query key is not present. Defaults to `random`.
	FallbackRaw json.RawMessage `json:"fallback,omitempty" caddy:"namespace=http.reverse_proxy.selection_policies inline_key=policy"`
	fallback    Selector
}

// CaddyModule returns the Caddy module information.
func (QueryHashSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.query",
		New: func() caddy.Module { return new(QueryHashSelection) },
	}
}

// Provision sets up the module.
func (s *QueryHashSelection) Provision(ctx caddy.Context) error {
	if s.Key == "" {
		return fmt.Errorf("query key is required")
	}
	if s.FallbackRaw == nil {
		s.FallbackRaw = caddyconfig.JSONModuleObject(RandomSelection{}, "policy", "random", nil)
	}
	mod, err := ctx.LoadModule(s, "FallbackRaw")
	if err != nil {
		return fmt.Errorf("loading fallback selection policy: %s", err)
	}
	s.fallback = mod.(Selector)
	return nil
}

// Select returns an available host, if any.
func (s QueryHashSelection) Select(pool UpstreamPool, req *http.Request, _ http.ResponseWriter) *Upstream {
	// Since the query may have multiple values for the same key,
	// we'll join them to avoid a problem where the user can control
	// the upstream that the request goes to by sending multiple values
	// for the same key, when the upstream only considers the first value.
	// Keep in mind that a client changing the order of the values may
	// affect which upstream is selected, but this is a semantically
	// different request, because the order of the values is significant.
	vals := strings.Join(req.URL.Query()[s.Key], ",")
	if vals == "" {
		return s.fallback.Select(pool, req, nil)
	}
	return hostByHashing(pool, vals)
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (s *QueryHashSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume policy name

	if !d.NextArg() {
		return d.ArgErr()
	}
	s.Key = d.Val()

	for d.NextBlock(0) {
		switch d.Val() {
		case "fallback":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if s.FallbackRaw != nil {
				return d.Err("fallback selection policy already specified")
			}
			mod, err := loadFallbackPolicy(d)
			if err != nil {
				return err
			}
			s.FallbackRaw = mod
		default:
			return d.Errf("unrecognized option '%s'", d.Val())
		}
	}
	return nil
}

// HeaderHashSelection is a policy that selects
// a host based on a given request header.
type HeaderHashSelection struct {
	// The HTTP header field whose value is to be hashed and used for upstream selection.
	Field string `json:"field,omitempty"`

	// The fallback policy to use if the header is not present. Defaults to `random`.
	FallbackRaw json.RawMessage `json:"fallback,omitempty" caddy:"namespace=http.reverse_proxy.selection_policies inline_key=policy"`
	fallback    Selector
}

// CaddyModule returns the Caddy module information.
func (HeaderHashSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.header",
		New: func() caddy.Module { return new(HeaderHashSelection) },
	}
}

// Provision sets up the module.
func (s *HeaderHashSelection) Provision(ctx caddy.Context) error {
	if s.Field == "" {
		return fmt.Errorf("header field is required")
	}
	if s.FallbackRaw == nil {
		s.FallbackRaw = caddyconfig.JSONModuleObject(RandomSelection{}, "policy", "random", nil)
	}
	mod, err := ctx.LoadModule(s, "FallbackRaw")
	if err != nil {
		return fmt.Errorf("loading fallback selection policy: %s", err)
	}
	s.fallback = mod.(Selector)
	return nil
}

// Select returns an available host, if any.
func (s HeaderHashSelection) Select(pool UpstreamPool, req *http.Request, _ http.ResponseWriter) *Upstream {
	// The Host header should be obtained from the req.Host field
	// since net/http removes it from the header map.
	if s.Field == "Host" && req.Host != "" {
		return hostByHashing(pool, req.Host)
	}

	val := req.Header.Get(s.Field)
	if val == "" {
		return s.fallback.Select(pool, req, nil)
	}
	return hostByHashing(pool, val)
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (s *HeaderHashSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume policy name

	if !d.NextArg() {
		return d.ArgErr()
	}
	s.Field = d.Val()

	for d.NextBlock(0) {
		switch d.Val() {
		case "fallback":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if s.FallbackRaw != nil {
				return d.Err("fallback selection policy already specified")
			}
			mod, err := loadFallbackPolicy(d)
			if err != nil {
				return err
			}
			s.FallbackRaw = mod
		default:
			return d.Errf("unrecognized option '%s'", d.Val())
		}
	}
	return nil
}

// CookieHashSelection is a policy that selects
// a host based on a given cookie name.
type CookieHashSelection struct {
	// The HTTP cookie name whose value is to be hashed and used for upstream selection.
	Name string `json:"name,omitempty"`
	// Secret to hash (Hmac256) chosen upstream in cookie
	Secret string `json:"secret,omitempty"`
	// The cookie's Max-Age before it expires. Default is no expiry.
	MaxAge caddy.Duration `json:"max_age,omitempty"`

	// The fallback policy to use if the cookie is not present. Defaults to `random`.
	FallbackRaw json.RawMessage `json:"fallback,omitempty" caddy:"namespace=http.reverse_proxy.selection_policies inline_key=policy"`
	fallback    Selector
}

// CaddyModule returns the Caddy module information.
func (CookieHashSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.cookie",
		New: func() caddy.Module { return new(CookieHashSelection) },
	}
}

// Provision sets up the module.
func (s *CookieHashSelection) Provision(ctx caddy.Context) error {
	if s.Name == "" {
		s.Name = "lb"
	}
	if s.FallbackRaw == nil {
		s.FallbackRaw = caddyconfig.JSONModuleObject(RandomSelection{}, "policy", "random", nil)
	}
	mod, err := ctx.LoadModule(s, "FallbackRaw")
	if err != nil {
		return fmt.Errorf("loading fallback selection policy: %s", err)
	}
	s.fallback = mod.(Selector)
	return nil
}

// Select returns an available host, if any.
func (s CookieHashSelection) Select(pool UpstreamPool, req *http.Request, w http.ResponseWriter) *Upstream {
	// selects a new Host using the fallback policy (typically random)
	// and write a sticky session cookie to the response.
	selectNewHost := func() *Upstream {
		upstream := s.fallback.Select(pool, req, w)
		if upstream == nil {
			return nil
		}
		sha, err := hashCookie(s.Secret, upstream.Dial)
		if err != nil {
			return upstream
		}
		cookie := &http.Cookie{
			Name:   s.Name,
			Value:  sha,
			Path:   "/",
			Secure: false,
		}
		isProxyHttps := false
		if trusted, ok := caddyhttp.GetVar(req.Context(), caddyhttp.TrustedProxyVarKey).(bool); ok && trusted {
			xfp, xfpOk, _ := lastHeaderValue(req.Header, "X-Forwarded-Proto")
			isProxyHttps = xfpOk && xfp == "https"
		}
		if req.TLS != nil || isProxyHttps {
			cookie.Secure = true
			cookie.SameSite = http.SameSiteNoneMode
		}
		if s.MaxAge > 0 {
			cookie.MaxAge = int(time.Duration(s.MaxAge).Seconds())
		}
		http.SetCookie(w, cookie)
		return upstream
	}

	cookie, err := req.Cookie(s.Name)
	// If there's no cookie, select a host using the fallback policy
	if err != nil || cookie == nil {
		return selectNewHost()
	}
	// If the cookie is present, loop over the available upstreams until we find a match
	cookieValue := cookie.Value
	for _, upstream := range pool {
		if !upstream.Available() {
			continue
		}
		sha, err := hashCookie(s.Secret, upstream.Dial)
		if err == nil && sha == cookieValue {
			return upstream
		}
	}
	// If there is no matching host, select a host using the fallback policy
	return selectNewHost()
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens. Syntax:
//
//	lb_policy cookie [<name> [<secret>]] {
//		fallback <policy>
//		max_age <duration>
//	}
//
// By default name is `lb`
func (s *CookieHashSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	args := d.RemainingArgs()
	switch len(args) {
	case 1:
	case 2:
		s.Name = args[1]
	case 3:
		s.Name = args[1]
		s.Secret = args[2]
	default:
		return d.ArgErr()
	}
	for d.NextBlock(0) {
		switch d.Val() {
		case "fallback":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if s.FallbackRaw != nil {
				return d.Err("fallback selection policy already specified")
			}
			mod, err := loadFallbackPolicy(d)
			if err != nil {
				return err
			}
			s.FallbackRaw = mod
		case "max_age":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if s.MaxAge != 0 {
				return d.Err("cookie max_age already specified")
			}
			maxAge, err := caddy.ParseDuration(d.Val())
			if err != nil {
				return d.Errf("invalid duration: %s", d.Val())
			}
			if maxAge <= 0 {
				return d.Errf("invalid duration: %s, max_age should be non-zero and positive", d.Val())
			}
			if d.NextArg() {
				return d.ArgErr()
			}
			s.MaxAge = caddy.Duration(maxAge)
		default:
			return d.Errf("unrecognized option '%s'", d.Val())
		}
	}
	return nil
}

// hashCookie hashes (HMAC 256) some data with the secret
func hashCookie(secret string, data string) (string, error) {
	h := hmac.New(sha256.New, []byte(secret))
	_, err := h.Write([]byte(data))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// selectRandomHost returns a random available host
func selectRandomHost(pool []*Upstream) *Upstream {
	// use reservoir sampling because the number of available
	// hosts isn't known: https://en.wikipedia.org/wiki/Reservoir_sampling
	var randomHost *Upstream
	var count int
	for _, upstream := range pool {
		if !upstream.Available() {
			continue
		}
		// (n % 1 == 0) holds for all n, therefore a
		// upstream will always be chosen if there is at
		// least one available
		count++
		if (weakrand.Int() % count) == 0 { //nolint:gosec
			randomHost = upstream
		}
	}
	return randomHost
}

// leastRequests returns the host with the
// least number of active requests to it.
// If more than one host has the same
// least number of active requests, then
// one of those is chosen at random.
func leastRequests(upstreams []*Upstream) *Upstream {
	if len(upstreams) == 0 {
		return nil
	}
	var best []*Upstream
	bestReqs := -1
	for _, upstream := range upstreams {
		if upstream == nil {
			continue
		}
		reqs := upstream.NumRequests()
		if reqs == 0 {
			return upstream
		}
		// If bestReqs was just initialized to -1
		// we need to append upstream also
		if reqs <= bestReqs || bestReqs == -1 {
			bestReqs = reqs
			best = append(best, upstream)
		}
	}
	if len(best) == 0 {
		return nil
	}
	if len(best) == 1 {
		return best[0]
	}
	return best[weakrand.Intn(len(best))] //nolint:gosec
}

// hostByHashing returns an available host from pool based on a hashable string s.
func hostByHashing(pool []*Upstream, s string) *Upstream {
	// Highest Random Weight (HRW, or "Rendezvous") hashing,
	// guarantees stability when the list of upstreams changes;
	// see https://medium.com/i0exception/rendezvous-hashing-8c00e2fb58b0,
	// https://randorithms.com/2020/12/26/rendezvous-hashing.html,
	// and https://en.wikipedia.org/wiki/Rendezvous_hashing.
	var highestHash uint64
	var upstream *Upstream
	for _, up := range pool {
		if !up.Available() {
			continue
		}
		h := hash(up.String() + s) // important to hash key and server together
		if h > highestHash {
			highestHash = h
			upstream = up
		}
	}
	return upstream
}

// hash calculates a fast hash based on s.
func hash(s string) uint64 {
	h := xxhash.New()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func loadFallbackPolicy(d *caddyfile.Dispenser) (json.RawMessage, error) {
	name := d.Val()
	modID := "http.reverse_proxy.selection_policies." + name
	unm, err := caddyfile.UnmarshalModule(d, modID)
	if err != nil {
		return nil, err
	}
	sel, ok := unm.(Selector)
	if !ok {
		return nil, d.Errf("module %s (%T) is not a reverseproxy.Selector", modID, unm)
	}
	return caddyconfig.JSONModuleObject(sel, "policy", name, nil), nil
}

// MementoSelection is a policy that selects a host
// using the BinomialHash algorithm for optimal load distribution
// and minimal redistribution when the topology changes.
type MementoSelection struct {
	// The field to use for hashing. Can be "ip", "uri", "header", etc.
	// Defaults to "ip" if not specified.
	Field string `json:"field,omitempty"`

	// The header field name if Field is "header"
	HeaderField string `json:"header_field,omitempty"`

	// The fallback policy to use if the field is not present. Defaults to `random`.
	FallbackRaw json.RawMessage `json:"fallback,omitempty" caddy:"namespace=http.reverse_proxy.selection_policies inline_key=policy"`
	fallback    Selector

	// Internal state for consistent hashing
	consistentEngine *memento.ConsistentEngine
	topology         sync.Map // Track which nodes are currently available (map[string]bool, thread-safe)

	// Event system integration
	events *caddyevents.App
	ctx    caddy.Context
}

// CaddyModule returns the Caddy module information.
func (MementoSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.memento",
		New: func() caddy.Module { return new(MementoSelection) },
	}
}

// Provision sets up the module.
func (s *MementoSelection) Provision(ctx caddy.Context) error {
	if s.Field == "" {
		s.Field = "ip" // Default to IP-based hashing
	}

	if s.FallbackRaw == nil {
		s.FallbackRaw = caddyconfig.JSONModuleObject(RandomSelection{}, "policy", "random", nil)
	}
	mod, err := ctx.LoadModule(s, "FallbackRaw")
	if err != nil {
		return fmt.Errorf("loading fallback selection policy: %s", err)
	}
	s.fallback = mod.(Selector)

	// Initialize topology tracking (sync.Map is zero-initialized, no need to initialize)

	// Initialize consistent engine
	// ConsistentEngine creates MementoEngine internally, which in turn creates BinomialEngine
	// This architecture allows for arbitrary node removals while maintaining consistency
	s.consistentEngine = memento.NewConsistentEngine()

	// Set up event system integration
	s.ctx = ctx

	// Note: Event subscription will be handled by the reverse proxy handler
	// when it provisions this selection policy, not during our Provision phase

	return nil
}

// Select returns an available host, if any.
func (s MementoSelection) Select(pool UpstreamPool, req *http.Request, w http.ResponseWriter) *Upstream {
	if len(pool) == 0 {
		return nil
	}

	// Get the key to hash based on the field type
	var key string
	switch s.Field {
	case "ip":
		clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			clientIP = req.RemoteAddr
		}
		key = clientIP
	case "client_ip":
		address := caddyhttp.GetVar(req.Context(), caddyhttp.ClientIPVarKey).(string)
		clientIP, _, err := net.SplitHostPort(address)
		if err != nil {
			clientIP = address
		}
		key = clientIP
	case "uri":
		key = req.RequestURI
	case "header":
		if s.HeaderField == "" {
			return s.fallback.Select(pool, req, w)
		}
		if s.HeaderField == "Host" && req.Host != "" {
			key = req.Host
		} else {
			key = req.Header.Get(s.HeaderField)
		}
		if key == "" {
			return s.fallback.Select(pool, req, w)
		}
	default:
		return s.fallback.Select(pool, req, w)
	}

	// Use consistent engine with Memento for stable hashing (default)
	// If the engine is not yet initialized with topology (e.g., no events in tests),
	// fall back to random selection.
	var bucket int
	var nodeID string

	// No lock needed: Indirection and topology are now thread-safe (sync.Map)
	if s.consistentEngine == nil || s.consistentEngine.Size() == 0 {
		// Fallback: use random selection if engine not initialized
		return s.fallback.Select(pool, req, w)
	}

	// Get bucket from consistent engine (thread-safe: Indirection uses sync.Map)
	bucket = s.consistentEngine.GetBucket(key)

	// Convert bucket index to node ID (string)
	// The bucket index is an index in the MementoEngine, not in the pool
	nodeID = s.consistentEngine.GetNodeID(bucket)
	if nodeID == "" {
		// Bucket index doesn't map to a valid node - this shouldn't happen
		// but we fallback to random selection to be safe
		return s.fallback.Select(pool, req, w)
	}

	// Find the Upstream in the pool that matches this node ID
	// We need to search by comparing the String() representation
	for _, upstream := range pool {
		if upstream.String() == nodeID {
			return upstream
		}
	}

	// Node ID not found in pool - this can happen if topology and pool are out of sync
	// Fallback to random selection
	return s.fallback.Select(pool, req, w)
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (s *MementoSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume policy name

	// Parse field type
	if d.NextArg() {
		s.Field = d.Val()
	}

	for d.NextBlock(0) {
		switch d.Val() {
		case "field":
			if !d.NextArg() {
				return d.ArgErr()
			}
			s.Field = d.Val()
		case "header_field":
			if !d.NextArg() {
				return d.ArgErr()
			}
			s.HeaderField = d.Val()
		case "fallback":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if s.FallbackRaw != nil {
				return d.Err("fallback selection policy already specified")
			}
			mod, err := loadFallbackPolicy(d)
			if err != nil {
				return err
			}
			s.FallbackRaw = mod
		default:
			return d.Errf("unrecognized option '%s'", d.Val())
		}
	}
	return nil
}

// SetEventsApp sets the events app for this selection policy
// This should be called by the reverse proxy handler when it provisions the selection policy
func (s *MementoSelection) SetEventsApp(events *caddyevents.App) {
	if events != nil {
		s.events = events
		s.subscribeToHealthEvents()
	}
}

// PopulateInitialTopology populates the Memento topology with initial upstreams
// This should be called by the reverse proxy handler after SetEventsApp
func (s *MementoSelection) PopulateInitialTopology(upstreams []*Upstream) {
	if s.consistentEngine == nil {
		return
	}

	// No lock needed: topology and engine are now thread-safe
	// Add all configured upstreams as healthy
	for _, upstream := range upstreams {
		host := upstream.String()
		if _, exists := s.topology.Load(host); !exists {
			if err := s.consistentEngine.AddNode(host); err != nil {
				// Log error but continue - this shouldn't happen in normal operation
				continue
			}
			s.topology.Store(host, true)
		}
	}
}

// subscribeToHealthEvents subscribes to health check events for real-time topology updates
func (s *MementoSelection) subscribeToHealthEvents() {
	if s.events == nil {
		return
	}

	// Subscribe to "healthy" events
	s.events.On("healthy", s)

	// Subscribe to "unhealthy" events
	s.events.On("unhealthy", s)
}

// handleHealthyEvent handles when an upstream becomes healthy
func (s *MementoSelection) handleHealthyEvent(ctx context.Context, event caddy.Event) error {
	if s.consistentEngine == nil {
		return nil
	}

	host, ok := event.Data["host"].(string)
	if !ok {
		return nil
	}

	// No lock needed: topology and engine are now thread-safe
	// Add node to consistent engine if not already present
	if _, exists := s.topology.Load(host); !exists {
		if err := s.consistentEngine.AddNode(host); err != nil {
			// Log error but continue - this shouldn't happen in normal operation
			return nil
		}
		s.topology.Store(host, true)
	}

	return nil
}

// handleUnhealthyEvent handles when an upstream becomes unhealthy
func (s *MementoSelection) handleUnhealthyEvent(ctx context.Context, event caddy.Event) error {
	if s.consistentEngine == nil {
		return nil
	}

	host, ok := event.Data["host"].(string)
	if !ok {
		return nil
	}

	// No lock needed: topology and engine are now thread-safe
	// Remove node from consistent engine if present
	if _, exists := s.topology.Load(host); exists {
		if err := s.consistentEngine.RemoveNode(host); err != nil {
			// Log error but continue - node might have been already removed
			return nil
		}
		s.topology.Delete(host)
	}

	return nil
}

// Handle implements caddyevents.Handler interface
func (s *MementoSelection) Handle(ctx context.Context, event caddy.Event) error {
	switch event.Name() {
	case "healthy":
		return s.handleHealthyEvent(ctx, event)
	case "unhealthy":
		return s.handleUnhealthyEvent(ctx, event)
	}
	return nil
}

// WeightedMementoSelection is a policy that selects a host
// using a weighted consistent hashing algorithm (based on Memento)
// for optimal load distribution and minimal redistribution when the topology changes.
type WeightedMementoSelection struct {
	// The field to use for hashing. Can be "ip", "uri", "header", etc.
	// Defaults to "ip" if not specified.
	Field string `json:"field,omitempty"`

	// The header field name if Field is "header"
	HeaderField string `json:"header_field,omitempty"`

	// The weight of each upstream in order,
	// corresponding with the list of upstreams configured.
	Weights []int `json:"weights,omitempty"`

	// The fallback policy to use if the field is not present. Defaults to `random`.
	FallbackRaw json.RawMessage `json:"fallback,omitempty" caddy:"namespace=http.reverse_proxy.selection_policies inline_key=policy"`
	fallback    Selector

	// Internal state for consistent hashing
	consistentEngine *WeightedConsistentEngine
	initialWeights   map[*Upstream]int // Persist initial weights for re-balancing
	topology         sync.Map          // Track which nodes are currently available (map[*Upstream]bool, thread-safe)

	// Event system integration
	events *caddyevents.App
	ctx    caddy.Context
	logger *zap.Logger
}

// CaddyModule returns the Caddy module information.
func (WeightedMementoSelection) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.reverse_proxy.selection_policies.weighted_memento",
		New: func() caddy.Module { return new(WeightedMementoSelection) },
	}
}

// UnmarshalCaddyfile sets up the module from Caddyfile tokens.
func (s *WeightedMementoSelection) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // consume policy name

	// The first arguments are the weights
	s.Weights = []int{}
	for _, weight := range d.RemainingArgs() {
		// if an arg is not a number, it's probably a block opener
		if _, err := strconv.Atoi(weight); err != nil {
			break
		}
		d.NextArg() // consume the weight arg
		weightInt, err := strconv.Atoi(weight)
		if err != nil {
			return d.Errf("invalid weight value '%s': %v", weight, err)
		}
		if weightInt < 0 {
			return d.Errf("invalid weight value '%s': weight should be non-negative", weight)
		}
		s.Weights = append(s.Weights, weightInt)
	}

	for d.NextBlock(0) {
		switch d.Val() {
		case "field":
			if !d.NextArg() {
				return d.ArgErr()
			}
			s.Field = d.Val()
		case "header_field":
			if !d.NextArg() {
				return d.ArgErr()
			}
			s.HeaderField = d.Val()
		case "fallback":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if s.FallbackRaw != nil {
				return d.Err("fallback selection policy already specified")
			}
			mod, err := loadFallbackPolicy(d)
			if err != nil {
				return err
			}
			s.FallbackRaw = mod
		default:
			return d.Errf("unrecognized option '%s'", d.Val())
		}
	}
	return nil
}

// Provision sets up the module.
func (s *WeightedMementoSelection) Provision(ctx caddy.Context) error {
	s.logger = ctx.Logger(s)
	if s.Field == "" {
		s.Field = "ip" // Default to IP-based hashing
	}

	if s.FallbackRaw == nil {
		s.FallbackRaw = caddyconfig.JSONModuleObject(RandomSelection{}, "policy", "random", nil)
	}
	mod, err := ctx.LoadModule(s, "FallbackRaw")
	if err != nil {
		return fmt.Errorf("loading fallback selection policy: %s", err)
	}
	s.fallback = mod.(Selector)

	// Initialize the weighted consistent hashing engine
	s.consistentEngine = NewWeightedConsistentEngine()
	s.initialWeights = make(map[*Upstream]int)

	// Set up event system integration
	s.ctx = ctx

	return nil
}

// Select returns an available host, if any.
func (s *WeightedMementoSelection) Select(pool UpstreamPool, req *http.Request, w http.ResponseWriter) *Upstream {
	if len(pool) == 0 {
		return nil
	}

	// Get the key to hash based on the field type
	var key string
	switch s.Field {
	case "ip":
		clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			clientIP = req.RemoteAddr
		}
		key = clientIP
	case "client_ip":
		address := caddyhttp.GetVar(req.Context(), caddyhttp.ClientIPVarKey).(string)
		clientIP, _, err := net.SplitHostPort(address)
		if err != nil {
			clientIP = address
		}
		key = clientIP
	case "uri":
		key = req.RequestURI
	case "header":
		if s.HeaderField == "" {
			return s.fallback.Select(pool, req, w)
		}
		if s.HeaderField == "Host" && req.Host != "" {
			key = req.Host
		} else {
			key = req.Header.Get(s.HeaderField)
		}
		if key == "" {
			return s.fallback.Select(pool, req, w)
		}
	default:
		return s.fallback.Select(pool, req, w)
	}

	// Use the weighted consistent engine to find the node
	if upstream, ok := s.consistentEngine.Lookup(key); ok {
		// s.logger.Debug("memento lookup", zap.String("key", key), zap.String("nodeID", upstream.String()))

		if upstream.Available() {
			return upstream
		}

	}

	// Fallback if the node is not found or engine is not ready
	return s.fallback.Select(pool, req, w)
}

// SetEventsApp sets the events app for this selection policy
func (s *WeightedMementoSelection) SetEventsApp(events *caddyevents.App) {
	if events != nil {
		s.events = events
		s.subscribeToHealthEvents()
	}
}

// PopulateInitialTopology populates the Memento topology with initial upstreams
func (s *WeightedMementoSelection) PopulateInitialTopology(upstreams []*Upstream) {
	if s.consistentEngine == nil {
		return
	}

	nodesWithWeights := make(map[*Upstream]int)
	for i, upstream := range upstreams {
		weight := 1 // Default weight
		if i < len(s.Weights) {
			weight = s.Weights[i]
		}
		if weight <= 0 {
			continue
		}
		nodesWithWeights[upstream] = weight
		s.initialWeights[upstream] = weight // Store for later
	}

	s.consistentEngine.InitCluster(nodesWithWeights)

	// Mark all nodes as present in the topology map
	for upstream := range nodesWithWeights {
		s.topology.Store(upstream, true)
	}
}

// subscribeToHealthEvents subscribes to health check events for real-time topology updates
func (s *WeightedMementoSelection) subscribeToHealthEvents() {
	if s.events == nil {
		return
	}
	s.events.On("healthy", s)
	s.events.On("unhealthy", s)
}

// handleHealthyEvent handles when an upstream becomes healthy
func (s *WeightedMementoSelection) handleHealthyEvent(ctx context.Context, event caddy.Event) error {
	if s.consistentEngine == nil {
		return nil
	}
	host, ok := event.Data["host"].(string)
	if !ok {
		return nil
	}

	var upstream *Upstream
	for up := range s.initialWeights {
		if up.String() == host {
			upstream = up
			break
		}
	}
	if upstream == nil {
		return nil
	}

	// Re-add the node only if it was previously part of the topology
	if _, exists := s.topology.Load(upstream); !exists {
		weight, hasInitialWeight := s.initialWeights[upstream]
		if !hasInitialWeight {
			// This should not happen if the host was part of the initial config
			weight = 1
		}
		s.consistentEngine.AddNode(upstream, weight)
		s.topology.Store(upstream, true)
	}
	return nil
}

// handleUnhealthyEvent handles when an upstream becomes unhealthy
func (s *WeightedMementoSelection) handleUnhealthyEvent(ctx context.Context, event caddy.Event) error {
	if s.consistentEngine == nil {
		return nil
	}
	host, ok := event.Data["host"].(string)
	if !ok {
		return nil
	}

	var upstream *Upstream
	for up := range s.initialWeights {
		if up.String() == host {
			upstream = up
			break
		}
	}
	if upstream == nil {
		return nil
	}

	if _, exists := s.topology.Load(upstream); exists {
		s.consistentEngine.RemoveNode(upstream)
		s.topology.Delete(upstream)
	}
	return nil
}

// Handle implements caddyevents.Handler interface
func (s *WeightedMementoSelection) Handle(ctx context.Context, event caddy.Event) error {
	switch event.Name() {
	case "healthy":
		return s.handleHealthyEvent(ctx, event)
	case "unhealthy":
		return s.handleUnhealthyEvent(ctx, event)
	}
	return nil
}

// Interface guards
var (
	_ Selector = (*RandomSelection)(nil)
	_ Selector = (*RandomChoiceSelection)(nil)
	_ Selector = (*LeastConnSelection)(nil)
	_ Selector = (*RoundRobinSelection)(nil)
	_ Selector = (*WeightedRoundRobinSelection)(nil)
	_ Selector = (*FirstSelection)(nil)
	_ Selector = (*IPHashSelection)(nil)
	_ Selector = (*ClientIPHashSelection)(nil)
	_ Selector = (*URIHashSelection)(nil)
	_ Selector = (*QueryHashSelection)(nil)
	_ Selector = (*HeaderHashSelection)(nil)
	_ Selector = (*CookieHashSelection)(nil)
	_ Selector = (*MementoSelection)(nil)
	_ Selector = (*WeightedMementoSelection)(nil)

	_ caddy.Validator = (*RandomChoiceSelection)(nil)

	_ caddy.Provisioner = (*RandomChoiceSelection)(nil)
	_ caddy.Provisioner = (*WeightedRoundRobinSelection)(nil)
	_ caddy.Provisioner = (*MementoSelection)(nil)
	_ caddy.Provisioner = (*WeightedMementoSelection)(nil)

	_ caddyfile.Unmarshaler = (*RandomChoiceSelection)(nil)
	_ caddyfile.Unmarshaler = (*WeightedRoundRobinSelection)(nil)
	_ caddyfile.Unmarshaler = (*MementoSelection)(nil)
	_ caddyfile.Unmarshaler = (*WeightedMementoSelection)(nil)

	_ caddyevents.Handler = (*MementoSelection)(nil)
	_ caddyevents.Handler = (*WeightedMementoSelection)(nil)

	// Back-compat alias
)

// Backward compatibility: keep old name as alias
var ()
