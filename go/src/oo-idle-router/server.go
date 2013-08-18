package main

import (
  "fmt"
  "sync"
  "strconv"
  "net/http"
  "net/http/httputil"
  "os"
  "time"
  "runtime"
  "flag"
  "bytes"
  "os/exec"
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
  go func(){
    // wait until someone is listening to activate the backend
    select {
    case b.waits<- b.ready:
      //fmt.Println("  ", "Now processing", b.Target)
      backends.Activate(b)
      fmt.Println("  ", "Waiting for activators", b.Target)
    }

    for {
      done := false
      select {
      case b.waits<- b.ready:
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

  time.Sleep(3 * time.Second)

  fmt.Println("  ", "Activated  "+b.Target)
  backends.Replace(b.Copy(&now))
  close(b.ready)
}

func (b *InactiveBackend) Serve(res http.ResponseWriter, req *http.Request) {
  if b.error != nil {
    res.WriteHeader(503)
    res.Write([]byte("Unable to activate backend: "+*b.error))
  } else {
    //fmt.Println("  ", "Proxying "+req.Host)
    b.server.ServeHTTP(res, req)
  }
}


type Backends struct {
  mutex sync.Mutex
  gears map[string]Backend
  activate chan *InactiveBackend  
}

func (b *Backends) Activate(backend *InactiveBackend) {
  b.activate <- backend
}

func (b *Backends) Start(workers int) {
  b.activate = make(chan *InactiveBackend)
  for i:=0; i<workers; i++ {
    go func() {
      select {
      case backend := <-b.activate:
        fmt.Println("  ", "Processing a backend on a worker")
        backend.Activate()
      }
    }()
  }
}

func (b *Backends) Add(host string, backendHost string, backendPort int) {
  b.mutex.Lock(); defer b.mutex.Unlock()
  var backend = NewInactiveBackend([]string{host}, backendHost, backendPort)
  b.gears[host] = backend
}
func (b *Backends) Replace(backend Backend) {
  b.mutex.Lock(); defer b.mutex.Unlock()
  for _, host := range backend.Hosts() {
    b.gears[host] = backend
  }
}
func (b *Backends) For(host string) (Backend, bool) {
  b.mutex.Lock(); defer b.mutex.Unlock()
  backend, ok := b.gears[host]
  return backend, ok
}


type ActivationDisabled string
func (e ActivationDisabled) Error() string {
    return string(e)
}
type Commands struct {
  activate string
}
func (c *Commands) Init() {
  c.activate = *flag.String("activate", "", "The command to use to activate an idled backend")
  fmt.Println("Activate command =", c.activate)
}
func (c *Commands) Activate(backend Backend) (string, error) {
  if len(c.activate) == 0 {
    return "<activate disabled>", ActivationDisabled("Backends will not be activated")
  }

  cmd := exec.Command(c.activate, backend.Id(), backend.Hosts()[0])
  var out bytes.Buffer
  err := cmd.Run()
  return out.String(), err
}

var backends = Backends{gears: make(map[string]Backend)}
var commands = Commands{}
var activateTimeout = 15 * time.Second
var startTime = time.Now()

func main() {
  fmt.Printf("GOMAXPROCS is %d\n", runtime.GOMAXPROCS(0))

  commands.Init()

  //var message = 
  backends.Add("port22003.rhcloud.com", "localhost", 22003)
  backends.Start(1)

  http.HandleFunc("/", proxy)
  var on = os.Getenv("HOST")+":"+os.Getenv("PORT")
  fmt.Println("listening to "+on+"...")
  if err := http.ListenAndServe(on, nil); err != nil {
    panic(err)
  }
}

func proxy(res http.ResponseWriter, req *http.Request) {
  if backend, ok := backends.For(req.Host); ok {
    select {
    case _, open := <-backend.Active():
      if open {
        errorTimeout(res, req)
      } else {
        backend.Serve(res, req)
      }
    }
  } else {
    backendNotFound(res, req)
  }
}

func backendNotFound(res http.ResponseWriter, req *http.Request) {
  res.WriteHeader(404)
  res.Write([]byte("Not found"))
}

func errorWhileActivating(res http.ResponseWriter, req *http.Request) {
  res.WriteHeader(503)
  res.Write([]byte("Unable to activate backend"))
}

func errorTimeout(res http.ResponseWriter, req *http.Request) {
  res.WriteHeader(503)
  res.Write([]byte("Timeout on backend"))
}