package main

import (
	"net/http"
	"net/http/httputil"
	"strconv"
	"sync"
	"time"
)

type Reason int

const (
	None Reason = iota
	Ready
	Timeout
	Dropped
	QueueFull
)

type Route interface {
	Id() string
	Hosts() []string
	Ready(ActivateFunc) Reason
	Serve(http.ResponseWriter, *http.Request)
}

type BaseRoute struct {
	id       string
	hosts    []string
	server   http.Handler
	awake_at *time.Time
}

type IdleRoute struct {
	BaseRoute
	queue  chan bool
	ready  chan bool
	reason Reason
	mutex  sync.Mutex
}

type ActiveRoute struct {
	BaseRoute
}

type ActivateFunc func(r *IdleRoute) bool

func NewRoute(hosts []string, routeHost string, routePort int) *IdleRoute {
	var spec = routeHost + ":" + strconv.Itoa(routePort)
	var route = IdleRoute{
		BaseRoute: BaseRoute{
			spec,
			hosts,
			&httputil.ReverseProxy{
				Director: func(req *http.Request) {
					req.URL.Scheme = "http"
					req.URL.Host = spec
				},
				Transport: &http.Transport{
					Proxy:                 http.ProxyFromEnvironment,
					MaxIdleConnsPerHost:   maxIdleConnsPerRoute,
					ResponseHeaderTimeout: maxRouteResponseHeaderTimeout,
				},
			},
			nil,
		},
		queue: make(chan bool, 4),
		ready: make(chan bool),
	}
	return &route
}

func (r *BaseRoute) Id() string {
	return r.id
}

func (r *BaseRoute) Hosts() []string {
	return r.hosts
}

func (r *IdleRoute) idle() bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.awake_at == nil
}

func (r *IdleRoute) Ready(activate ActivateFunc) Reason {
	select {
	case r.queue <- true:
	default:
		return QueueFull
	}

	if r.idle() {
		if !activate(r) {
			return QueueFull
		}
	}

	select {
	case <-r.ready:
	}
	return r.reason
}

func (r *IdleRoute) Wake() *ActiveRoute {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.awake_at != nil {
		return nil
	}
	now := time.Now()
	r.awake_at = &now
	return &ActiveRoute{r.BaseRoute}
}

func (r *IdleRoute) Serve(res http.ResponseWriter, req *http.Request) {
	if r.reason != Ready {
		res.WriteHeader(503)
		res.Write([]byte("Unable to activate route: " + strconv.Itoa(int(r.reason))))
		return
	}
	r.server.ServeHTTP(res, req)
}

func (r *ActiveRoute) Ready(activate ActivateFunc) Reason {
	return Ready
}

func (r *ActiveRoute) Serve(res http.ResponseWriter, req *http.Request) {
	r.server.ServeHTTP(res, req)
}
