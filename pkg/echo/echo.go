package echo

import (
	"io"
	"net/http"
	"strings"
)

func Echo(w http.ResponseWriter, r *http.Request) {
	args := strings.Split(r.URL.String(), "/")
	io.WriteString(w, args[len(args)-1]+"!")
}
