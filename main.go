package main

import (
	"net/http"
	"net/http/cgi"
	"os/exec"
)

// Leaves out error handling for clarity
func cgiHandler(w http.ResponseWriter, r *http.Request) {
	// Parse name from post request
	r.ParseForm()
	name := r.FormValue("name")

	// Execute the CGI script with the name as an environment variable
	cmd := exec.Command("cgi-script.sh")
	cmd.Env = append(cmd.Env, "name="+name)

	// Serve the CGI script
	handler := cgi.Handler{Path: cmd.Path, Dir: cmd.Dir, Env: cmd.Env}
	handler.ServeHTTP(w, r)
}

func main() {
	http.HandleFunc("/", cgiHandler)
	http.ListenAndServe("localhost:8080", nil)
}
