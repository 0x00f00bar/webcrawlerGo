package main

import (
	"fmt"
	"net/http"
)

type envelope map[string]any

const readMaxRequestBytes = 1024

func (app *webapp) logError(r *http.Request, err error) {
	app.Logger.Print(err, "; request_method: "+r.Method, ", request_url: "+r.URL.String())
}

func (app *webapp) notFoundResponse(w http.ResponseWriter, r *http.Request) {
	message := "the requested resource could not be found"
	app.errorResponse(w, r, http.StatusNotFound, message)
}

func (app *webapp) methodNotAllowedResponse(w http.ResponseWriter, r *http.Request) {
	message := fmt.Sprintf("the %s method is not supported for this resource", r.Method)
	app.errorResponse(w, r, http.StatusMethodNotAllowed, message)
}

func (app *webapp) errorResponse(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	message any,
) {
	env := envelope{"error": message}

	err := app.writeJSON(w, status, env, nil)
	if err != nil {
		app.logError(r, err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (app *webapp) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logError(r, err)

	message := "the server encountered a problem and could not process your request"
	app.errorResponse(w, r, http.StatusInternalServerError, message)
}
