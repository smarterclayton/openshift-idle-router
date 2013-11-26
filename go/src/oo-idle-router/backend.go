package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"
	"time"
)

type Backend interface {
	Id() string
	Hosts() []string
	Active() chan bool
	Serve(http.ResponseWriter, *http.Request)
}

type InactiveBackend struct {
	Target    string
	hosts     []string
	server    http.Handler
	waits     chan chan bool
	ready     chan bool
	activated *time.Time
	error     *string
}

type ActiveBackend struct {
	*InactiveBackend
}

func NewInactiveBackend(hosts []string, backendHost string, backendPort int) *InactiveBackend {
	var spec = backendHost + ":" + strconv.Itoa(backendPort)
	var backend = InactiveBackend{
		spec,
		hosts,
		&httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL.Scheme = "http"
				req.URL.Host = spec
			},
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				MaxIdleConnsPerHost:   maxIdleConnsPerBackend,
				ResponseHeaderTimeout: maxBackendResponseHeaderTimeout,
			},
		},
		make(chan chan bool),
		make(chan bool),
		&time.Time{},
		nil,
	}
	backend.init()
	return &backend
}

func (b *InactiveBackend) init() {
	go func() {
		// wait until someone is listening to activate the backend
		select {
		case b.waits <- b.ready:
			//fmt.Println("  ", "Now processing", b.Target)
			backends.Activate(b)
			fmt.Println("  ", "Waiting for activators", b.Target)
		}

		for {
			done := false
			select {
			case b.waits <- b.ready:
			case <-time.After(activateTimeout):
				done = true
			}
			if done {
				break
			}
		}
		fmt.Println("  ", "Processing complete")
	}()
}

func (b InactiveBackend) Active() chan bool {
	//fmt.Println("  ", "Waiting for backend", b.Target)
	return <-b.waits
}

func (b ActiveBackend) Active() chan bool {
	//fmt.Println("  ", "Active", b.Target)
	return b.ready
}

func (b *InactiveBackend) Hosts() []string {
	return b.hosts
}

func (b *InactiveBackend) Id() string {
	return b.Target
}

func (b *InactiveBackend) Copy(at *time.Time) *ActiveBackend {
	active := ActiveBackend{b}
	active.waits = nil
	active.activated = at
	active.error = nil
	return &active
}

func (b *InactiveBackend) Activate() {
	if !b.activated.IsZero() {
		fmt.Println("WARN", b.Target, "Already activated")
		return
	}

	var now = time.Now()

	fmt.Println("  ", "Activating "+b.Target)

	if out, err := commands.Activate(b); err != nil {
		fmt.Println("  ", "No activation:", err.Error())
	} else {
		fmt.Println("  ", out)
	}

	//time.Sleep(3 * time.Second)

	fmt.Println("  ", "Activated  "+b.Target)
	backends.Replace(b.Copy(&now))
	close(b.ready)
}

func (b *InactiveBackend) Serve(res http.ResponseWriter, req *http.Request) {
	if b.error != nil {
		res.WriteHeader(503)
		res.Write([]byte("Unable to activate backend: " + *b.error))
	} else {
		//fmt.Println("  ", "Proxying "+req.Host)
		b.server.ServeHTTP(res, req)
	}
}
