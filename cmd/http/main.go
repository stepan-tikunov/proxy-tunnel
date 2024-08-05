package main

import (
	"fmt"
	"html"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})

	panic(http.ListenAndServe(":8080", nil))
}
