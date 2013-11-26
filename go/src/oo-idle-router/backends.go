package main

import (
	"fmt"
	"sync"
)

type Backends struct {
	mutex    sync.RWMutex
	gears    map[string]Backend
	activate chan *InactiveBackend
}

func (b *Backends) Activate(backend *InactiveBackend) {
	b.activate <- backend
}

func (b *Backends) Start(workers int) {
	b.activate = make(chan *InactiveBackend)
	for i := 0; i < workers; i++ {
		go func() {
			select {
			case backend := <-b.activate:
				fmt.Println("  ", "Processing a backend on a worker")
				backend.Activate()
			}
		}()
	}
}

func (b *Backends) active() map[string]Backend {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return b.gears
}

func (b *Backends) copy() map[string]Backend {
	active := b.active()
	replace := make(map[string]Backend)
	for key, value := range active {
		replace[key] = value
	}
	return replace
}

func (b *Backends) swap(replace map[string]Backend) map[string]Backend {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.gears = replace
	return replace
}

func (b *Backends) Add(host string, backendHost string, backendPort int) {
	replace := b.copy()
	var backend = NewInactiveBackend([]string{host}, backendHost, backendPort)
	replace[host] = backend
	b.swap(replace)
}
func (b *Backends) Replace(backend Backend) {
	replace := b.copy()
	for _, host := range backend.Hosts() {
		replace[host] = backend
	}
	b.swap(replace)
}
func (b *Backends) For(host string) (Backend, bool) {
	backend, ok := b.active()[host]
	return backend, ok
}
