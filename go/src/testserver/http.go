package main

import (
  "fmt"
  "net/http"
  "os"
  "flag"
)

func main() {
  var message = *flag.String("message", "hello world ("+os.Getenv("PORT")+")", "A message to display when returning")
  http.HandleFunc("/",
    func(res http.ResponseWriter, req *http.Request) {
      fmt.Fprintln(res, message)
    })
  fmt.Println("listening...")
  err := http.ListenAndServe(os.Getenv("HOST")+":"+os.Getenv("PORT"), nil)
  if err != nil {
    panic(err)
  }
}


