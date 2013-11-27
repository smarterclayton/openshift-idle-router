package main

import (
	"sync"
)

type Routes struct {
	gears map[string]Route
	mutex sync.RWMutex
}

func (r *Routes) Add(host string, routeHost string, routePort int) {
	replace := r.copy()
	route := NewRoute([]string{host}, routeHost, routePort)
	replace[host] = route
	r.swap(replace)
}

func (r *Routes) Replace(route Route) {
	replace := r.copy()
	for _, host := range route.Hosts() {
		replace[host] = route
	}
	r.swap(replace)
}

func (r *Routes) For(host string) (Route, bool) {
	route, ok := r.active()[host]
	return route, ok
}

func (r *Routes) active() map[string]Route {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.gears
}

func (r *Routes) copy() map[string]Route {
	active := r.active()
	replace := make(map[string]Route)
	for key, value := range active {
		replace[key] = value
	}
	return replace
}

func (r *Routes) swap(replace map[string]Route) map[string]Route {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.gears = replace
	return replace
}
