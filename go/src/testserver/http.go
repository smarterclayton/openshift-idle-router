package main

import (
  "fmt"
  "net/http"
  "os"
  "flag"
  "strconv"
)

func main() {
  portString := os.Getenv("PORT")
  port := 8080

  if len(portString) != 0 {
    envPort, err := strconv.Atoi(portString)
    if err != nil {
      panic(err)
    }
    port = envPort
  }

  flag.IntVar(&port, "p", port, "The port to listen on")
  var message = *flag.String("message", "", "A message to display when returning")
  flag.Parse()

  if len(message) == 0 {
    message = "hello world ("+strconv.Itoa(port)+")"    
  }

  http.HandleFunc("/",
    func(res http.ResponseWriter, req *http.Request) {
      fmt.Fprintln(res, message)
    })
  fmt.Println("listening...")
  err := http.ListenAndServe(os.Getenv("HOST")+":"+strconv.Itoa(port), nil)
  if err != nil {
    panic(err)
  }
}


