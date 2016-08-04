package streamer

import (
	"errors"
	"net/http"
)

// getParameter returns the value of a given GET parameter in a request object
func getValFromRequest(param string, r *http.Request) (v string, err error) {
	e := r.ParseForm()
	if e != nil {
		err = errors.New("Bad query string.")
		return v, err
	}
	v = r.Form.Get(param)

	if v != "" {
		return v, err
	}

	err = errors.New("Get parameter not found.")
	return v, err
}
