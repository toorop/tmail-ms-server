package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/codegangsta/negroni"
	"github.com/golang/protobuf/proto"
	"github.com/julienschmidt/httprouter"
	"github.com/nbio/httpcontext"
)

// main launches HTTP server
func main() {
	router := httprouter.New()

	// Routes

	// Home
	router.GET("/", wrapHandler(hHome))

	// helo/ehlo
	router.POST("/helo", wrapHandler(hHelo))

	// Server
	n := negroni.New(negroni.NewRecovery(), negroni.NewLogger())
	n.UseHandler(router)
	addr := fmt.Sprintf("127.0.0.1:3333")
	log.Fatalln(http.ListenAndServe(addr, n))
}

// hHome home handler
func hHome(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Welcome to tmail microservice server"))
}

// hHelo helo handler
func hHelo(w http.ResponseWriter, r *http.Request) {
	// Get rawdata
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}
	helo := &Helo{}
	err = proto.Unmarshal(data, helo)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}

	smtpResponse := &SmtpResponseBase{
		SmtpCode: proto.Int32(250),
		SmtpMsg:  proto.String(""),
	}

	if helo.GetHelo() == "spambot" {
		smtpResponse.SmtpCode = proto.Int32(550)
		smtpResponse.SmtpMsg = proto.String("smtp access denied")
	}

	data, err = proto.Marshal(smtpResponse)
	w.Write(data)
}

// wrapHandler puts httprouter.Params in query context
// in order to keep compatibily with http.Handler
func wrapHandler(h func(http.ResponseWriter, *http.Request)) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		httpcontext.Set(r, "params", ps)
		h(w, r)
	}
}
