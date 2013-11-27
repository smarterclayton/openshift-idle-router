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

type WakeDisabled string

func (e WakeDisabled) Error() string {
	return string(e)
}

type Commands struct {
}

func (c *Commands) Wake(route Route) (string, error) {
	if len(commandWake) == 0 {
		return "<wake disabled>", WakeDisabled("Routes will not be woken")
	}

	cmd := exec.Command(commandWake, route.Id(), route.Hosts()[0])
	var out bytes.Buffer
	err := cmd.Run()
	return out.String(), err
}

func wakeRoute(r *IdleRoute) bool {
	select {
	case pendingWakes <- r:
	default:
		return false
	}
	return true
}

func StartWorkers(workers int) {
	for i := 0; i < workers; i++ {
		go func() {
			for route := range pendingWakes {
				active := route.Wake()
				if active == nil {
					fmt.Println("WARN", route.Id(), "Already awake")
					continue
				}
				fmt.Println("  ", "Worker", "Activating route", route.Id())

				if out, err := commands.Wake(route); err != nil {
					fmt.Println("  ", "Worker", "No activation:", err.Error())
				} else {
					fmt.Println("  ", out)
				}
				time.Sleep(3 * time.Second)
				route.reason = Ready

				fmt.Println("  ", "Worker", route.Id())
				routes.Replace(active)
				fmt.Println("  ", "Worker", "New route is now live", route.Id())

				go func() {
					done := false
					for {
						select {
						case route.ready <- true:
							fmt.Println("  ", "Worker", "Satisfied route.ready", route.Id())
						case <-route.queue:
							fmt.Println("  ", "Worker", "Satisfied route.queue", route.Id())
						default:
							fmt.Println("  ", "Worker", "Queues done", route.Id())
							close(route.ready)
							done = true
						}
						if done {
							break
						}
					}
				}()
			}
		}()
	}
}

func proxy(res http.ResponseWriter, req *http.Request) {
	route, ok := routes.For(req.Host)
	if !ok {
		routeNotFound(res, req)
		return
	}
	result := route.Ready(wakeRoute)
	if result == Ready {
		route.Serve(res, req)
	} else {
		fmt.Println("  ", "proxy", "Failed", req.Host, result)
		errorTimeout(res, req)
	}
}

func routeNotFound(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(404)
	res.Write([]byte("Not found"))
}

func errorWhileActivating(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(503)
	res.Write([]byte("Unable to wake route"))
}

func errorTimeout(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(503)
	res.Write([]byte("Timeout on route"))
}

func init() {
	flag.StringVar(&commandWake, "activate", "", "The command to use to wake an idled route")
}

var routes = Routes{
	gears: make(map[string]Route),
}

var commands = Commands{}
var activateTimeout = 3 * time.Second
var startTime = time.Now()
var pendingWakes = make(chan *IdleRoute, 4)
var commandWake = ""
var maxIdleConnsPerRoute = 16
var maxRouteResponseHeaderTimeout = 30 * time.Second
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	fmt.Printf("GOMAXPROCS is %d\n", runtime.GOMAXPROCS(0))
	flag.Parse()
	fmt.Println("Wake", commandWake)
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
	StartWorkers(1)
	routes.Add("port22003.rhcloud.com", "localhost", 22003)

	http.HandleFunc("/", proxy)
	var on = os.Getenv("HOST") + ":" + os.Getenv("PORT")
	fmt.Println("listening to " + on + "...")
	if err := http.ListenAndServe(on, nil); err != nil {
		panic(err)
	}
}
