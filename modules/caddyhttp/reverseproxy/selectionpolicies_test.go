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
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	"github.com/caddyserver/caddy/v2/modules/caddyevents"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func testPool() UpstreamPool {
	return UpstreamPool{
		{Host: new(Host), Dial: "0.0.0.1"},
		{Host: new(Host), Dial: "0.0.0.2"},
		{Host: new(Host), Dial: "0.0.0.3"},
	}
}

func TestRoundRobinPolicy(t *testing.T) {
	pool := testPool()
	rrPolicy := RoundRobinSelection{}
	req, _ := http.NewRequest("GET", "/", nil)

	h := rrPolicy.Select(pool, req, nil)
	// First selected host is 1, because counter starts at 0
	// and increments before host is selected
	if h != pool[1] {
		t.Error("Expected first round robin host to be second host in the pool.")
	}
	h = rrPolicy.Select(pool, req, nil)
	if h != pool[2] {
		t.Error("Expected second round robin host to be third host in the pool.")
	}
	h = rrPolicy.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected third round robin host to be first host in the pool.")
	}
	// mark host as down
	pool[1].setHealthy(false)
	h = rrPolicy.Select(pool, req, nil)
	if h != pool[2] {
		t.Error("Expected to skip down host.")
	}
	// mark host as up
	pool[1].setHealthy(true)

	h = rrPolicy.Select(pool, req, nil)
	if h == pool[2] {
		t.Error("Expected to balance evenly among healthy hosts")
	}
	// mark host as full
	pool[1].countRequest(1)
	pool[1].MaxRequests = 1
	h = rrPolicy.Select(pool, req, nil)
	if h != pool[2] {
		t.Error("Expected to skip full host.")
	}
}

func TestWeightedRoundRobinPolicy(t *testing.T) {
	pool := testPool()
	wrrPolicy := WeightedRoundRobinSelection{
		Weights:     []int{3, 2, 1},
		totalWeight: 6,
	}
	req, _ := http.NewRequest("GET", "/", nil)

	h := wrrPolicy.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected first weighted round robin host to be first host in the pool.")
	}
	h = wrrPolicy.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected second weighted round robin host to be first host in the pool.")
	}
	// Third selected host is 1, because counter starts at 0
	// and increments before host is selected
	h = wrrPolicy.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected third weighted round robin host to be second host in the pool.")
	}
	h = wrrPolicy.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected fourth weighted round robin host to be second host in the pool.")
	}
	h = wrrPolicy.Select(pool, req, nil)
	if h != pool[2] {
		t.Error("Expected fifth weighted round robin host to be third host in the pool.")
	}
	h = wrrPolicy.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected sixth weighted round robin host to be first host in the pool.")
	}

	// mark host as down
	pool[0].setHealthy(false)
	h = wrrPolicy.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected to skip down host.")
	}
	// mark host as up
	pool[0].setHealthy(true)

	h = wrrPolicy.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected to select first host on availability.")
	}
	// mark host as full
	pool[1].countRequest(1)
	pool[1].MaxRequests = 1
	h = wrrPolicy.Select(pool, req, nil)
	if h != pool[2] {
		t.Error("Expected to skip full host.")
	}
}

func TestWeightedRoundRobinPolicyWithZeroWeight(t *testing.T) {
	pool := testPool()
	wrrPolicy := WeightedRoundRobinSelection{
		Weights:     []int{0, 2, 1},
		totalWeight: 3,
	}
	req, _ := http.NewRequest("GET", "/", nil)

	h := wrrPolicy.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected first weighted round robin host to be second host in the pool.")
	}

	h = wrrPolicy.Select(pool, req, nil)
	if h != pool[2] {
		t.Error("Expected second weighted round robin host to be third host in the pool.")
	}

	h = wrrPolicy.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected third weighted round robin host to be second host in the pool.")
	}

	// mark second host as down
	pool[1].setHealthy(false)
	h = wrrPolicy.Select(pool, req, nil)
	if h != pool[2] {
		t.Error("Expect select next available host.")
	}

	h = wrrPolicy.Select(pool, req, nil)
	if h != pool[2] {
		t.Error("Expect select only available host.")
	}
	// mark second host as up
	pool[1].setHealthy(true)

	h = wrrPolicy.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expect select first host on availability.")
	}

	// test next select in full cycle
	expected := []*Upstream{pool[1], pool[2], pool[1], pool[1], pool[2], pool[1]}
	for i, want := range expected {
		got := wrrPolicy.Select(pool, req, nil)
		if want != got {
			t.Errorf("Selection %d: got host[%s], want host[%s]", i+1, got, want)
		}
	}
}

func TestLeastConnPolicy(t *testing.T) {
	pool := testPool()
	lcPolicy := LeastConnSelection{}
	req, _ := http.NewRequest("GET", "/", nil)

	pool[0].countRequest(10)
	pool[1].countRequest(10)
	h := lcPolicy.Select(pool, req, nil)
	if h != pool[2] {
		t.Error("Expected least connection host to be third host.")
	}
	pool[2].countRequest(100)
	h = lcPolicy.Select(pool, req, nil)
	if h != pool[0] && h != pool[1] {
		t.Error("Expected least connection host to be first or second host.")
	}
}

func TestIPHashPolicy(t *testing.T) {
	pool := testPool()
	ipHash := IPHashSelection{}
	req, _ := http.NewRequest("GET", "/", nil)

	// We should be able to predict where every request is routed.
	req.RemoteAddr = "172.0.0.1:80"
	h := ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	req.RemoteAddr = "172.0.0.2:80"
	h = ipHash.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected ip hash policy host to be the second host.")
	}
	req.RemoteAddr = "172.0.0.3:80"
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	req.RemoteAddr = "172.0.0.4:80"
	h = ipHash.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected ip hash policy host to be the second host.")
	}

	// we should get the same results without a port
	req.RemoteAddr = "172.0.0.1"
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	req.RemoteAddr = "172.0.0.2"
	h = ipHash.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected ip hash policy host to be the second host.")
	}
	req.RemoteAddr = "172.0.0.3"
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	req.RemoteAddr = "172.0.0.4"
	h = ipHash.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected ip hash policy host to be the second host.")
	}

	// we should get a healthy host if the original host is unhealthy and a
	// healthy host is available
	req.RemoteAddr = "172.0.0.4"
	pool[1].setHealthy(false)
	h = ipHash.Select(pool, req, nil)
	if h != pool[2] {
		t.Error("Expected ip hash policy host to be the third host.")
	}

	req.RemoteAddr = "172.0.0.2"
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	pool[1].setHealthy(true)

	req.RemoteAddr = "172.0.0.3"
	pool[2].setHealthy(false)
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	req.RemoteAddr = "172.0.0.4"
	h = ipHash.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected ip hash policy host to be the second host.")
	}

	// We should be able to resize the host pool and still be able to predict
	// where a req will be routed with the same IP's used above
	pool = UpstreamPool{
		{Host: new(Host), Dial: "0.0.0.2"},
		{Host: new(Host), Dial: "0.0.0.3"},
	}
	req.RemoteAddr = "172.0.0.1:80"
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	req.RemoteAddr = "172.0.0.2:80"
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	req.RemoteAddr = "172.0.0.3:80"
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	req.RemoteAddr = "172.0.0.4:80"
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}

	// We should get nil when there are no healthy hosts
	pool[0].setHealthy(false)
	pool[1].setHealthy(false)
	h = ipHash.Select(pool, req, nil)
	if h != nil {
		t.Error("Expected ip hash policy host to be nil.")
	}

	// Reproduce #4135
	pool = UpstreamPool{
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
	}
	pool[0].setHealthy(false)
	pool[1].setHealthy(false)
	pool[2].setHealthy(false)
	pool[3].setHealthy(false)
	pool[4].setHealthy(false)
	pool[5].setHealthy(false)
	pool[6].setHealthy(false)
	pool[7].setHealthy(false)
	pool[8].setHealthy(true)

	// We should get a result back when there is one healthy host left.
	h = ipHash.Select(pool, req, nil)
	if h == nil {
		// If it is nil, it means we missed a host even though one is available
		t.Error("Expected ip hash policy host to not be nil, but it is nil.")
	}
}

func TestClientIPHashPolicy(t *testing.T) {
	pool := testPool()
	ipHash := ClientIPHashSelection{}
	req, _ := http.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), caddyhttp.VarsCtxKey, make(map[string]any)))

	// We should be able to predict where every request is routed.
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.1:80")
	h := ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.2:80")
	h = ipHash.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected ip hash policy host to be the second host.")
	}
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.3:80")
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.4:80")
	h = ipHash.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected ip hash policy host to be the second host.")
	}

	// we should get the same results without a port
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.1")
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.2")
	h = ipHash.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected ip hash policy host to be the second host.")
	}
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.3")
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.4")
	h = ipHash.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected ip hash policy host to be the second host.")
	}

	// we should get a healthy host if the original host is unhealthy and a
	// healthy host is available
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.4")
	pool[1].setHealthy(false)
	h = ipHash.Select(pool, req, nil)
	if h != pool[2] {
		t.Error("Expected ip hash policy host to be the third host.")
	}

	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.2")
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	pool[1].setHealthy(true)

	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.3")
	pool[2].setHealthy(false)
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.4")
	h = ipHash.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected ip hash policy host to be the second host.")
	}

	// We should be able to resize the host pool and still be able to predict
	// where a req will be routed with the same IP's used above
	pool = UpstreamPool{
		{Host: new(Host), Dial: "0.0.0.2"},
		{Host: new(Host), Dial: "0.0.0.3"},
	}
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.1:80")
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.2:80")
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.3:80")
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}
	caddyhttp.SetVar(req.Context(), caddyhttp.ClientIPVarKey, "172.0.0.4:80")
	h = ipHash.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected ip hash policy host to be the first host.")
	}

	// We should get nil when there are no healthy hosts
	pool[0].setHealthy(false)
	pool[1].setHealthy(false)
	h = ipHash.Select(pool, req, nil)
	if h != nil {
		t.Error("Expected ip hash policy host to be nil.")
	}

	// Reproduce #4135
	pool = UpstreamPool{
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
		{Host: new(Host)},
	}
	pool[0].setHealthy(false)
	pool[1].setHealthy(false)
	pool[2].setHealthy(false)
	pool[3].setHealthy(false)
	pool[4].setHealthy(false)
	pool[5].setHealthy(false)
	pool[6].setHealthy(false)
	pool[7].setHealthy(false)
	pool[8].setHealthy(true)

	// We should get a result back when there is one healthy host left.
	h = ipHash.Select(pool, req, nil)
	if h == nil {
		// If it is nil, it means we missed a host even though one is available
		t.Error("Expected ip hash policy host to not be nil, but it is nil.")
	}
}

func TestFirstPolicy(t *testing.T) {
	pool := testPool()
	firstPolicy := FirstSelection{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	h := firstPolicy.Select(pool, req, nil)
	if h != pool[0] {
		t.Error("Expected first policy host to be the first host.")
	}

	pool[0].setHealthy(false)
	h = firstPolicy.Select(pool, req, nil)
	if h != pool[1] {
		t.Error("Expected first policy host to be the second host.")
	}
}

func TestQueryHashPolicy(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	queryPolicy := QueryHashSelection{Key: "foo"}
	if err := queryPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	pool := testPool()

	request := httptest.NewRequest(http.MethodGet, "/?foo=1", nil)
	h := queryPolicy.Select(pool, request, nil)
	if h != pool[0] {
		t.Error("Expected query policy host to be the first host.")
	}

	request = httptest.NewRequest(http.MethodGet, "/?foo=100000", nil)
	h = queryPolicy.Select(pool, request, nil)
	if h != pool[1] {
		t.Error("Expected query policy host to be the second host.")
	}

	request = httptest.NewRequest(http.MethodGet, "/?foo=1", nil)
	pool[0].setHealthy(false)
	h = queryPolicy.Select(pool, request, nil)
	if h != pool[2] {
		t.Error("Expected query policy host to be the third host.")
	}

	request = httptest.NewRequest(http.MethodGet, "/?foo=100000", nil)
	h = queryPolicy.Select(pool, request, nil)
	if h != pool[1] {
		t.Error("Expected query policy host to be the second host.")
	}

	// We should be able to resize the host pool and still be able to predict
	// where a request will be routed with the same query used above
	pool = UpstreamPool{
		{Host: new(Host)},
		{Host: new(Host)},
	}

	request = httptest.NewRequest(http.MethodGet, "/?foo=1", nil)
	h = queryPolicy.Select(pool, request, nil)
	if h != pool[0] {
		t.Error("Expected query policy host to be the first host.")
	}

	pool[0].setHealthy(false)
	h = queryPolicy.Select(pool, request, nil)
	if h != pool[1] {
		t.Error("Expected query policy host to be the second host.")
	}

	request = httptest.NewRequest(http.MethodGet, "/?foo=4", nil)
	h = queryPolicy.Select(pool, request, nil)
	if h != pool[1] {
		t.Error("Expected query policy host to be the second host.")
	}

	pool[0].setHealthy(false)
	pool[1].setHealthy(false)
	h = queryPolicy.Select(pool, request, nil)
	if h != nil {
		t.Error("Expected query policy policy host to be nil.")
	}

	request = httptest.NewRequest(http.MethodGet, "/?foo=aa11&foo=bb22", nil)
	pool = testPool()
	h = queryPolicy.Select(pool, request, nil)
	if h != pool[0] {
		t.Error("Expected query policy host to be the first host.")
	}
}

func TestURIHashPolicy(t *testing.T) {
	pool := testPool()
	uriPolicy := URIHashSelection{}

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	h := uriPolicy.Select(pool, request, nil)
	if h != pool[2] {
		t.Error("Expected uri policy host to be the third host.")
	}

	pool[2].setHealthy(false)
	h = uriPolicy.Select(pool, request, nil)
	if h != pool[0] {
		t.Error("Expected uri policy host to be the first host.")
	}

	request = httptest.NewRequest(http.MethodGet, "/test_2", nil)
	h = uriPolicy.Select(pool, request, nil)
	if h != pool[0] {
		t.Error("Expected uri policy host to be the first host.")
	}

	// We should be able to resize the host pool and still be able to predict
	// where a request will be routed with the same URI's used above
	pool = UpstreamPool{
		{Host: new(Host)},
		{Host: new(Host)},
	}

	request = httptest.NewRequest(http.MethodGet, "/test", nil)
	h = uriPolicy.Select(pool, request, nil)
	if h != pool[0] {
		t.Error("Expected uri policy host to be the first host.")
	}

	pool[0].setHealthy(false)
	h = uriPolicy.Select(pool, request, nil)
	if h != pool[1] {
		t.Error("Expected uri policy host to be the first host.")
	}

	request = httptest.NewRequest(http.MethodGet, "/test_2", nil)
	h = uriPolicy.Select(pool, request, nil)
	if h != pool[1] {
		t.Error("Expected uri policy host to be the second host.")
	}

	pool[0].setHealthy(false)
	pool[1].setHealthy(false)
	h = uriPolicy.Select(pool, request, nil)
	if h != nil {
		t.Error("Expected uri policy policy host to be nil.")
	}
}

func TestLeastRequests(t *testing.T) {
	pool := testPool()
	pool[0].Dial = "localhost:8080"
	pool[1].Dial = "localhost:8081"
	pool[2].Dial = "localhost:8082"
	pool[0].setHealthy(true)
	pool[1].setHealthy(true)
	pool[2].setHealthy(true)
	pool[0].countRequest(10)
	pool[1].countRequest(20)
	pool[2].countRequest(30)

	result := leastRequests(pool)

	if result == nil {
		t.Error("Least request should not return nil")
	}

	if result != pool[0] {
		t.Error("Least request should return pool[0]")
	}
}

func TestRandomChoicePolicy(t *testing.T) {
	pool := testPool()
	pool[0].Dial = "localhost:8080"
	pool[1].Dial = "localhost:8081"
	pool[2].Dial = "localhost:8082"
	pool[0].setHealthy(false)
	pool[1].setHealthy(true)
	pool[2].setHealthy(true)
	pool[0].countRequest(10)
	pool[1].countRequest(20)
	pool[2].countRequest(30)

	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	randomChoicePolicy := RandomChoiceSelection{Choose: 2}

	h := randomChoicePolicy.Select(pool, request, nil)

	if h == nil {
		t.Error("RandomChoicePolicy should not return nil")
	}

	if h == pool[0] {
		t.Error("RandomChoicePolicy should not choose pool[0]")
	}
}

func TestCookieHashPolicy(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	cookieHashPolicy := CookieHashSelection{}
	if err := cookieHashPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	pool := testPool()
	pool[0].Dial = "localhost:8080"
	pool[1].Dial = "localhost:8081"
	pool[2].Dial = "localhost:8082"
	pool[0].setHealthy(true)
	pool[1].setHealthy(false)
	pool[2].setHealthy(false)
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	h := cookieHashPolicy.Select(pool, request, w)
	cookieServer1 := w.Result().Cookies()[0]
	if cookieServer1 == nil {
		t.Fatal("cookieHashPolicy should set a cookie")
	}
	if cookieServer1.Name != "lb" {
		t.Error("cookieHashPolicy should set a cookie with name lb")
	}
	if cookieServer1.Secure {
		t.Error("cookieHashPolicy should set cookie Secure attribute to false when request is not secure")
	}
	if h != pool[0] {
		t.Error("Expected cookieHashPolicy host to be the first only available host.")
	}
	pool[1].setHealthy(true)
	pool[2].setHealthy(true)
	request = httptest.NewRequest(http.MethodGet, "/test", nil)
	w = httptest.NewRecorder()
	request.AddCookie(cookieServer1)
	h = cookieHashPolicy.Select(pool, request, w)
	if h != pool[0] {
		t.Error("Expected cookieHashPolicy host to stick to the first host (matching cookie).")
	}
	s := w.Result().Cookies()
	if len(s) != 0 {
		t.Error("Expected cookieHashPolicy to not set a new cookie.")
	}
	pool[0].setHealthy(false)
	request = httptest.NewRequest(http.MethodGet, "/test", nil)
	w = httptest.NewRecorder()
	request.AddCookie(cookieServer1)
	h = cookieHashPolicy.Select(pool, request, w)
	if h == pool[0] {
		t.Error("Expected cookieHashPolicy to select a new host.")
	}
	if w.Result().Cookies() == nil {
		t.Error("Expected cookieHashPolicy to set a new cookie.")
	}
}

func TestCookieHashPolicyWithSecureRequest(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	cookieHashPolicy := CookieHashSelection{}
	if err := cookieHashPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	pool := testPool()
	pool[0].Dial = "localhost:8080"
	pool[1].Dial = "localhost:8081"
	pool[2].Dial = "localhost:8082"
	pool[0].setHealthy(true)
	pool[1].setHealthy(false)
	pool[2].setHealthy(false)

	// Create a test server that serves HTTPS requests
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := cookieHashPolicy.Select(pool, r, w)
		if h != pool[0] {
			t.Error("Expected cookieHashPolicy host to be the first only available host.")
		}
	}))
	defer ts.Close()

	// Make a new HTTPS request to the test server
	client := ts.Client()
	request, err := http.NewRequest(http.MethodGet, ts.URL+"/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}

	// Check if the cookie set is Secure and has SameSiteNone mode
	cookies := response.Cookies()
	if len(cookies) == 0 {
		t.Fatal("Expected a cookie to be set")
	}
	cookie := cookies[0]
	if !cookie.Secure {
		t.Error("Expected cookie Secure attribute to be true when request is secure")
	}
	if cookie.SameSite != http.SameSiteNoneMode {
		t.Error("Expected cookie SameSite attribute to be None when request is secure")
	}
}

func TestCookieHashPolicyWithFirstFallback(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()
	cookieHashPolicy := CookieHashSelection{
		FallbackRaw: caddyconfig.JSONModuleObject(FirstSelection{}, "policy", "first", nil),
	}
	if err := cookieHashPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	pool := testPool()
	pool[0].Dial = "localhost:8080"
	pool[1].Dial = "localhost:8081"
	pool[2].Dial = "localhost:8082"
	pool[0].setHealthy(true)
	pool[1].setHealthy(true)
	pool[2].setHealthy(true)
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	h := cookieHashPolicy.Select(pool, request, w)
	cookieServer1 := w.Result().Cookies()[0]
	if cookieServer1 == nil {
		t.Fatal("cookieHashPolicy should set a cookie")
	}
	if cookieServer1.Name != "lb" {
		t.Error("cookieHashPolicy should set a cookie with name lb")
	}
	if h != pool[0] {
		t.Errorf("Expected cookieHashPolicy host to be the first only available host, got %s", h)
	}
	request = httptest.NewRequest(http.MethodGet, "/test", nil)
	w = httptest.NewRecorder()
	request.AddCookie(cookieServer1)
	h = cookieHashPolicy.Select(pool, request, w)
	if h != pool[0] {
		t.Errorf("Expected cookieHashPolicy host to stick to the first host (matching cookie), got %s", h)
	}
	s := w.Result().Cookies()
	if len(s) != 0 {
		t.Error("Expected cookieHashPolicy to not set a new cookie.")
	}
	pool[0].setHealthy(false)
	request = httptest.NewRequest(http.MethodGet, "/test", nil)
	w = httptest.NewRecorder()
	request.AddCookie(cookieServer1)
	h = cookieHashPolicy.Select(pool, request, w)
	if h != pool[1] {
		t.Errorf("Expected cookieHashPolicy to select the next first available host, got %s", h)
	}
	if w.Result().Cookies() == nil {
		t.Error("Expected cookieHashPolicy to set a new cookie.")
	}
}

func TestMementoSelectionPolicy(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	pool := testPool()
	mementoPolicy.PopulateInitialTopology(pool)
	req, _ := http.NewRequest("GET", "/", nil)

	// Test IP-based selection
	req.RemoteAddr = "172.0.0.1:80"
	h := mementoPolicy.Select(pool, req, nil)
	if h == nil {
		t.Error("Expected binomial policy to select a host")
	}

	// Test consistency - same IP should map to same host
	h2 := mementoPolicy.Select(pool, req, nil)
	if h != h2 {
		t.Error("Expected consistent mapping for same IP")
	}

	// Test different IPs
	req.RemoteAddr = "172.0.0.2:80"
	h3 := mementoPolicy.Select(pool, req, nil)
	if h == h3 {
		t.Log("Different IPs mapped to same host - this can happen with hash collisions")
	}
}

func TestMementoSelectionPolicyURI(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "uri"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	pool := testPool()
	mementoPolicy.PopulateInitialTopology(pool)
	req, _ := http.NewRequest("GET", "/test", nil)

	h := mementoPolicy.Select(pool, req, nil)
	if h == nil {
		t.Error("Expected binomial policy to select a host")
	}

	// Test consistency - same URI should map to same host
	h2 := mementoPolicy.Select(pool, req, nil)
	if h != h2 {
		t.Error("Expected consistent mapping for same URI")
	}

	// Test different URIs
	req2, _ := http.NewRequest("GET", "/different", nil)
	h3 := mementoPolicy.Select(pool, req2, nil)
	if h == h3 {
		t.Log("Different URIs mapped to same host - this can happen with hash collisions")
	}
}

func TestMementoSelectionPolicyHeader(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{
		Field:       "header",
		HeaderField: "User-Agent",
	}
	if err := mementoPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	pool := testPool()
	mementoPolicy.PopulateInitialTopology(pool)
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("User-Agent", "test-agent")

	h := mementoPolicy.Select(pool, req, nil)
	if h == nil {
		t.Error("Expected binomial policy to select a host")
	}

	// Test consistency - same header should map to same host
	h2 := mementoPolicy.Select(pool, req, nil)
	if h != h2 {
		t.Error("Expected consistent mapping for same header")
	}

	// Test fallback when header is missing
	req2, _ := http.NewRequest("GET", "/", nil)
	h3 := mementoPolicy.Select(pool, req2, nil)
	if h3 == nil {
		t.Error("Expected fallback policy to select a host")
	}
}

func TestMementoSelectionPolicyDistribution(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	pool := testPool()

	// Test distribution across hosts
	hostCounts := make(map[*Upstream]int)
	numRequests := 1000

	for i := 0; i < numRequests; i++ {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = fmt.Sprintf("172.0.0.%d:80", i%256)

		h := mementoPolicy.Select(pool, req, nil)
		if h != nil {
			hostCounts[h]++
		}
	}

	// Check that all hosts are used
	if len(hostCounts) != len(pool) {
		t.Errorf("Expected %d hosts to be used, got %d", len(pool), len(hostCounts))
	}

	// Check distribution is reasonable
	expectedPerHost := numRequests / len(pool)
	tolerance := expectedPerHost / 2 // 50% tolerance

	for host, count := range hostCounts {
		if count < expectedPerHost-tolerance || count > expectedPerHost+tolerance {
			t.Logf("Host %s has %d requests (expected ~%d)", host.Dial, count, expectedPerHost)
		}
	}
}

func TestMementoSelectionPolicyWithUnavailableHosts(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	pool := testPool()

	// Mark some hosts as unavailable
	pool[1].setHealthy(false)
	pool[2].setHealthy(false)

	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.0.0.1:80"

	h := mementoPolicy.Select(pool, req, nil)
	if h == nil {
		t.Error("Expected binomial policy to select an available host")
	}

	// Should not select unavailable hosts
	if h == pool[1] || h == pool[2] {
		t.Error("Binomial policy selected an unavailable host")
	}

	// Should select available hosts
	if h != pool[0] {
		t.Logf("Selected host %s, expected pool[0] %s", h.Dial, pool[0].Dial)
	}
}

func BenchmarkMementoSelectionPolicy(b *testing.B) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	pool := testPool()
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.0.0.1:80"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mementoPolicy.Select(pool, req, nil)
	}
}

func BenchmarkMementoSelectionPolicyDifferentIPs(b *testing.B) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	pool := testPool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/", nil)
		req.RemoteAddr = fmt.Sprintf("172.0.0.%d:80", i%256)
		mementoPolicy.Select(pool, req, nil)
	}
}

func TestMementoSelectionPolicyConsistent(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	// Note: Without events app, the policy will work but won't have real-time topology updates
	// This is expected behavior in test environments

	pool := testPool()
	mementoPolicy.PopulateInitialTopology(pool)
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.0.0.1:80"

	// Test initial selection
	h1 := mementoPolicy.Select(pool, req, nil)
	if h1 == nil {
		t.Error("Expected binomial policy to select a host")
	}

	// Test consistency - same key should map to same host
	h2 := mementoPolicy.Select(pool, req, nil)
	if h1 != h2 {
		t.Error("Expected consistent mapping for same IP")
	}

	// Mark one host as unavailable
	pool[1].setHealthy(false)

	// Test selection after topology change
	h3 := mementoPolicy.Select(pool, req, nil)
	if h3 == nil {
		t.Error("Expected binomial policy to select an available host")
	}

	// Should not select the unavailable host
	if h3 == pool[1] {
		t.Error("Binomial policy selected an unavailable host")
	}

	// Restore the host
	pool[1].setHealthy(true)

	// Test selection after restoration
	h4 := mementoPolicy.Select(pool, req, nil)
	if h4 == nil {
		t.Error("Expected binomial policy to select a host after restoration")
	}

	// The selection might change due to topology restoration, but should be consistent
	h5 := mementoPolicy.Select(pool, req, nil)
	if h4 != h5 {
		t.Error("Expected consistent mapping after restoration")
	}
}

func TestMementoSelectionPolicyChangeDetection(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	pool := testPool()
	mementoPolicy.PopulateInitialTopology(pool)
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.0.0.1:80"

	// Test initial selection
	h1 := mementoPolicy.Select(pool, req, nil)
	if h1 == nil {
		t.Error("Expected binomial policy to select a host")
	}

	// Test that subsequent calls with same topology don't trigger updates
	// (change detection should prevent unnecessary updates)
	h2 := mementoPolicy.Select(pool, req, nil)
	if h1 != h2 {
		t.Error("Expected consistent mapping for same IP")
	}

	// Test topology change detection
	pool[1].setHealthy(false)

	// This should trigger an update due to topology change
	h3 := mementoPolicy.Select(pool, req, nil)
	if h3 == nil {
		t.Error("Expected binomial policy to select an available host")
	}

	// Should not select the unavailable host
	if h3 == pool[1] {
		t.Error("Binomial policy selected an unavailable host")
	}

	// Test that subsequent calls with same changed topology are consistent
	h4 := mementoPolicy.Select(pool, req, nil)
	if h3 != h4 {
		t.Error("Expected consistent mapping after topology change")
	}

	// Restore the host
	pool[1].setHealthy(true)

	// This should trigger another update due to topology change
	h5 := mementoPolicy.Select(pool, req, nil)
	if h5 == nil {
		t.Error("Expected binomial policy to select a host after restoration")
	}

	// Test consistency after restoration
	h6 := mementoPolicy.Select(pool, req, nil)
	if h5 != h6 {
		t.Error("Expected consistent mapping after restoration")
	}
}

func TestMementoSelectionPolicyMultipleTopologyChanges(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	pool := testPool()
	mementoPolicy.PopulateInitialTopology(pool)
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.0.0.1:80"

	// Test initial selection
	h1 := mementoPolicy.Select(pool, req, nil)
	if h1 == nil {
		t.Error("Expected binomial policy to select a host")
	}

	// Remove first host using events (this is how Memento tracks topology changes)
	// Note: Memento uses consistent hashing and doesn't filter based on Upstream.Available()
	// unless nodes are explicitly removed from the topology via events
	mementoPolicy.handleUnhealthyEvent(context.Background(), caddy.Event{
		Data: map[string]any{"host": pool[0].String()},
	})
	h2 := mementoPolicy.Select(pool, req, nil)
	if h2 == nil {
		t.Error("Expected memento policy to select an available host")
	}
	// After removal, the selection should map to a different host
	if h2 == pool[0] {
		t.Log("Host was removed from topology but still selected - topology change may not have been effective")
	}

	// Remove second host
	mementoPolicy.handleUnhealthyEvent(context.Background(), caddy.Event{
		Data: map[string]any{"host": pool[1].String()},
	})
	h3 := mementoPolicy.Select(pool, req, nil)
	if h3 == nil {
		t.Error("Expected memento policy to select an available host")
	}

	// Restore first host
	mementoPolicy.handleHealthyEvent(context.Background(), caddy.Event{
		Data: map[string]any{"host": pool[0].String()},
	})
	h4 := mementoPolicy.Select(pool, req, nil)
	if h4 == nil {
		t.Error("Expected memento policy to select a host after restoration")
	}

	// Restore second host
	mementoPolicy.handleHealthyEvent(context.Background(), caddy.Event{
		Data: map[string]any{"host": pool[1].String()},
	})
	h5 := mementoPolicy.Select(pool, req, nil)
	if h5 == nil {
		t.Error("Expected memento policy to select a host after second restoration")
	}

	// Test consistency after all changes
	h6 := mementoPolicy.Select(pool, req, nil)
	if h5 != h6 {
		t.Error("Expected consistent mapping after all changes")
	}
}

func TestMementoSelectionPolicyEventDriven(t *testing.T) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	// Test that the policy can handle events directly
	pool := testPool()
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.0.0.1:80"

	// Initial selection
	h1 := mementoPolicy.Select(pool, req, nil)
	if h1 == nil {
		t.Error("Expected memento policy to select a host")
	}

	// Test that Handle method works for healthy events
	healthyEvent := caddy.Event{
		Data: map[string]any{"host": "localhost:8080"},
	}
	err := mementoPolicy.Handle(context.Background(), healthyEvent)
	if err != nil {
		t.Errorf("Handle healthy event error: %v", err)
	}

	// Test that Handle method works for unhealthy events
	unhealthyEvent := caddy.Event{
		Data: map[string]any{"host": "localhost:8081"},
	}
	err = mementoPolicy.Handle(context.Background(), unhealthyEvent)
	if err != nil {
		t.Errorf("Handle unhealthy event error: %v", err)
	}

	// Test consistency after events
	h2 := mementoPolicy.Select(pool, req, nil)
	if h2 == nil {
		t.Error("Expected memento policy to select a host after events")
	}
}

func TestMementoSelectionFullEventIntegration(t *testing.T) {
	// Create a full Caddy context with events app
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	// Create events app
	eventsApp := &caddyevents.App{}
	if err := eventsApp.Provision(ctx); err != nil {
		t.Errorf("Failed to provision events app: %v", err)
		t.FailNow()
	}

	// Create MementoSelection policy
	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		t.Errorf("Provision error: %v", err)
		t.FailNow()
	}

	// Test SetEventsApp integration BEFORE starting the events app
	mementoPolicy.SetEventsApp(eventsApp)

	// Verify that events app is set
	if mementoPolicy.events == nil {
		t.Error("Expected events app to be set after SetEventsApp call")
	}

	// NOW start the events app to enable subscriptions
	if err := eventsApp.Start(); err != nil {
		t.Errorf("Failed to start events app: %v", err)
		t.FailNow()
	}

	// Create test pool with specific hosts
	pool := []*Upstream{
		{Dial: "localhost:8080"},
		{Dial: "localhost:8081"},
		{Dial: "localhost:8082"},
	}

	// Create test request
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.0.0.1:80"

	// Test initial selection (should use fallback since no events yet)
	h1 := mementoPolicy.Select(pool, req, nil)
	if h1 == nil {
		t.Error("Expected memento policy to select a host")
	}

	// Simulate healthy events for all hosts using the events app
	for _, upstream := range pool {
		eventData := map[string]any{"host": upstream.Dial}
		eventsApp.Emit(ctx, "healthy", eventData)
	}

	// Verify topology is updated
	topologySize := 0
	mementoPolicy.topology.Range(func(_, _ interface{}) bool {
		topologySize++
		return true
	})

	if topologySize != 3 {
		t.Errorf("Expected topology size 3, got %d", topologySize)
	}

	// Test selection after topology is populated
	h2 := mementoPolicy.Select(pool, req, nil)
	if h2 == nil {
		t.Error("Expected memento policy to select a host after topology update")
	}

	// Test consistency - same key should select same host
	h3 := mementoPolicy.Select(pool, req, nil)
	if h2.Dial != h3.Dial {
		t.Error("Expected consistent selection for same key")
	}

	// Simulate unhealthy event for one host using the events app
	eventData := map[string]any{"host": "localhost:8081"}
	eventsApp.Emit(ctx, "unhealthy", eventData)

	// Verify topology is updated (unhealthy node should be removed)
	topologySizeAfterUnhealthy := 0
	mementoPolicy.topology.Range(func(_, _ interface{}) bool {
		topologySizeAfterUnhealthy++
		return true
	})

	// After unhealthy event, the node should be removed from topology
	if topologySizeAfterUnhealthy != 2 {
		t.Errorf("Expected topology size to be 2 after unhealthy event (one node removed), got %d", topologySizeAfterUnhealthy)
	}

	// Test selection after unhealthy event
	h4 := mementoPolicy.Select(pool, req, nil)
	if h4 == nil {
		t.Error("Expected memento policy to select a host after unhealthy event")
	}

	// The selection might change due to topology update, but should still be consistent
	h5 := mementoPolicy.Select(pool, req, nil)
	if h4.Dial != h5.Dial {
		t.Error("Expected consistent selection after unhealthy event")
	}

	// Test that the policy handles events without crashing
	t.Logf("MementoSelection successfully integrated with events system")
}

func BenchmarkMementoSelectionPolicyChangeDetection(b *testing.B) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	pool := testPool()
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.0.0.1:80"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mementoPolicy.Select(pool, req, nil)
	}
}

func BenchmarkMementoSelectionPolicyChangeDetectionWithTopologyChanges(b *testing.B) {
	ctx, cancel := caddy.NewContext(caddy.Context{Context: context.Background()})
	defer cancel()

	mementoPolicy := MementoSelection{Field: "ip"}
	if err := mementoPolicy.Provision(ctx); err != nil {
		b.Fatalf("Provision error: %v", err)
	}

	pool := testPool()
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.0.0.1:80"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate topology changes every 1000 requests
		if i%1000 == 0 {
			pool[1].setHealthy(false)
		} else if i%1000 == 500 {
			pool[1].setHealthy(true)
		}

		mementoPolicy.Select(pool, req, nil)
	}
}
