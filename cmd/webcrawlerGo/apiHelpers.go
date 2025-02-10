package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/0x00f00bar/webcrawlerGo/internal"
	"github.com/julienschmidt/httprouter"
)

type envelope map[string]any

const readMaxRequestBytes = 1024

func (app *webapp) logError(r *http.Request, err error) {
	app.Loggers.multiLogger.Print(
		err,
		"; request_method: "+r.Method,
		", request_url: "+r.URL.String(),
	)
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

func (app *webapp) badRequestResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.errorResponse(w, r, http.StatusBadRequest, err.Error())
}

func (app *webapp) failedValidationResponse(
	w http.ResponseWriter,
	r *http.Request,
	errors map[string]string,
) {
	app.errorResponse(w, r, http.StatusUnprocessableEntity, errors)
}

func (app *webapp) serverErrorResponse(w http.ResponseWriter, r *http.Request, err error) {
	app.logError(r, err)

	message := "the server encountered a problem and could not process your request"
	app.errorResponse(w, r, http.StatusInternalServerError, message)
}

func (app *webapp) editConflictResponse(w http.ResponseWriter, r *http.Request) {
	message := "unable to update the record due to edit conflict, please try again"
	app.errorResponse(w, r, http.StatusConflict, message)
}

func (app *webapp) readIDParam(r *http.Request) (int64, error) {
	params := httprouter.ParamsFromContext(r.Context())

	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil {
		return 0, errors.New("invalid id parameter")
	}

	return id, nil
}

func (app *webapp) writeJSON(
	w http.ResponseWriter,
	status int,
	data envelope,
	headers http.Header,
) error {
	js, err := json.Marshal(data)
	if err != nil {
		return err
	}

	js = append(js, '\n')
	for k, v := range headers {
		w.Header()[k] = v
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)
	return nil
}

func (app *webapp) readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	maxBytes := readMaxRequestBytes
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	jsDecoder := json.NewDecoder(r.Body)
	jsDecoder.DisallowUnknownFields()

	err := jsDecoder.Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf(
				"body contains badly-formed JSON (at character %d)",
				syntaxError.Offset,
			)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf(
					"body contains incorrect JSON type for field %q",
					unmarshalTypeError.Field,
				)
			}
			return fmt.Errorf(
				"body contains incorrect JSON type (at character %d)",
				unmarshalTypeError.Offset,
			)

		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("body contains unknown key %s", fieldName)

		case errors.As(err, &maxBytesError):
			return fmt.Errorf("body must not be larger than %d bytes", maxBytesError.Limit)

		case errors.As(err, &invalidUnmarshalError):
			panic(err)

		default:
			return err
		}
	}

	err = jsDecoder.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single JSON value")
	}
	return nil
}

func (app *webapp) readString(qs url.Values, key string, defaultValue string) string {
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}
	return s
}

func (app *webapp) readInt(
	qs url.Values,
	key string,
	defaultValue int,
	v *internal.Validator,
) int {
	s := qs.Get(key)
	if s == "" {
		return defaultValue
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		v.AddError(key, "must be an integer value")
		return defaultValue
	}

	return i
}

func (app *webapp) readBool(
	qs url.Values,
	key string,
	v *internal.Validator,
) (value bool, ok bool) {
	s := qs.Get(key)
	if s == "" {
		return
	}

	s = strings.ToLower(s)

	if !internal.PermittedValue(s, "true", "false") {
		v.AddError(key, "must be either 'true' or 'false'")
		return
	}

	if s == "true" {
		value, ok = true, true
		return
	}
	value, ok = false, true
	return
}

func (app *webapp) openBrowser(url string) {
	var cmd *exec.Cmd

	// Determine the OS and select the appropriate command to open the browser
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin": // macOS
		cmd = exec.Command("open", url)
	default: // Assume Linux or other Unix-like OS
		cmd = exec.Command("xdg-open", url)
	}

	err := cmd.Start()
	if err != nil {
		app.Loggers.multiLogger.Println("error opening browser: ", err)
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (app *webapp) logRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		preNextLog := fmt.Sprintf(
			"%s - \"%s %s %s\"",
			r.RemoteAddr,
			r.Method,
			r.RequestURI,
			r.Proto,
		)
		lrw := NewLoggingResponseWriter(w)
		next.ServeHTTP(lrw, r)
		app.Loggers.multiLogger.Printf("%s %d", preNextLog, lrw.statusCode)
	})
}
