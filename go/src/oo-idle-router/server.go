package main

import (
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

import _ "net/http/pprof"

type ActivationDisabled string

func (e ActivationDisabled) Error() string {
	return string(e)
}

type Commands struct {
}

func (c *Commands) Activate(backend Backend) (string, error) {
	if len(commandActivate) == 0 {
		return "<activate disabled>", ActivationDisabled("Backends will not be activated")
	}

	cmd := exec.Command(commandActivate, backend.Id(), backend.Hosts()[0])
	var out bytes.Buffer
	err := cmd.Run()
	return out.String(), err
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

func init() {
	flag.StringVar(&commandActivate, "activate", "", "The command to use to activate an idled backend")
}

var backends = Backends{gears: make(map[string]Backend)}
var commands = Commands{}
var activateTimeout = 15 * time.Second
var startTime = time.Now()
var commandActivate = ""
var maxIdleConnsPerBackend = 16
var maxBackendResponseHeaderTimeout = 30 * time.Second
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	fmt.Printf("GOMAXPROCS is %d\n", runtime.GOMAXPROCS(0))
	flag.Parse()
	fmt.Println("Activate", commandActivate)
	/*
	   if *cpuprofile != "" {
	     f, err := os.Create(*cpuprofile)
	     if err != nil {
	         log.Fatal(err)
	     }
	     pprof.StartCPUProfile(f)
	     defer pprof.StopCPUProfile()
	   }
	*/
	backends.Add("port22003.rhcloud.com", "localhost", 22003)
	backends.Start(1)

	http.HandleFunc("/", proxy)
	var on = os.Getenv("HOST") + ":" + os.Getenv("PORT")
	fmt.Println("listening to " + on + "...")
	if err := http.ListenAndServe(on, nil); err != nil {
		panic(err)
	}
}
