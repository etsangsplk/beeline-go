package hnyhttprouter

import (
	"net/http"
	"reflect"
	"runtime"
	"time"

	"github.com/honeycombio/beeline-go"
	"github.com/honeycombio/beeline-go/internal"
	"github.com/honeycombio/libhoney-go"
	"github.com/julienschmidt/httprouter"
)

// Middleware wraps httprouter handlers. Since it wraps handlers with explicit
// parameters, it can add those values to the event it generates.
func Middleware(handle httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		ctx := r.Context()
		// TODO find out if we're a sub-handler and don't stomp the parent
		// event, or at least get parent/child IDs and intentionally send a
		// subevent or something
		start := time.Now()
		ev := beeline.ContextEvent(ctx)
		if ev == nil {
			ev = libhoney.NewEvent()
			defer ev.Send()
			// put the event on the context for everybody downsteam to use
			r = r.WithContext(beeline.ContextWithEvent(ctx, ev))
		}
		// pull out any variables in the URL, add the thing we're matching, etc.
		for _, param := range ps {
			ev.AddField("handler.vars."+param.Key, param.Value)
		}
		// add some common fields from the request to our event
		internal.AddRequestProps(r, ev)
		// replace the writer with our wrapper to catch the status code
		wrappedWriter := internal.NewResponseWriter(w)
		name := runtime.FuncForPC(reflect.ValueOf(handle).Pointer()).Name()
		ev.AddField("handler.name", name)
		ev.AddField("name", name)

		handle(w, r, ps)

		if wrappedWriter.Status == 0 {
			wrappedWriter.Status = 200
		}
		ev.AddField("response.status_code", wrappedWriter.Status)
		ev.AddField("duration_ms", float64(time.Since(start))/float64(time.Millisecond))
		ev.Metadata, _ = ev.Fields()["name"]
	}
}
