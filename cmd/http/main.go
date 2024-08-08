package main

import (
	"fmt"
	"html"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		for range 200 {
			fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
		}
	})

	panic(http.ListenAndServe(":8080", nil))
}
