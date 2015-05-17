package main

import (
	//"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/codegangsta/negroni"
	"github.com/golang/protobuf/proto"
	"github.com/julienschmidt/httprouter"
	//"github.com/nbio/httpcontext"
)

// main launches HTTP server
func main() {
	router := httprouter.New()

	// Routes

	// Home
	router.GET("/", wrapHandler(hHome))

	// new smtpd client
	router.POST("/smtpdnewclient", wrapHandler(hNewSmtpdClient))

	// helo/ehlo
	//router.POST("/helo", wrapHandler(hHelo))

	// Server
	n := negroni.New(negroni.NewRecovery(), negroni.NewLogger())
	n.UseHandler(router)
	addr := fmt.Sprintf("127.0.0.1:3333")
	log.Fatalln(http.ListenAndServe(addr, n))
}

// hHome home handler
func hHome(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Welcome to tmail microservices server"))
}

// hNewSmtpdClient new smtpd client handler
func hNewSmtpdClient(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}
	newClientMsg := &SmtpdNewClientMsg{}
	err = proto.Unmarshal(data, newClientMsg)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}

	smtpResponse := &SmtpdResponse{
		SmtpCode: proto.Int32(220),
		SmtpMsg:  proto.String(""),
	}

	fmt.Println(newClientMsg.GetRemoteIp())

	if newClientMsg.GetRemoteIp() == "127.0.0.1" {
		smtpResponse.SmtpCode = proto.Int32(421)
		smtpResponse.SmtpMsg = proto.String("i'm sorry Z, i'm afraid i can't do that.")
		smtpResponse.CloseConnection = proto.Bool(true)
	}
	data, err = proto.Marshal(smtpResponse)
	w.Write(data)
}

// wrapHandler puts httprouter.Params in query context
// in order to keep compatibily with http.Handler
func wrapHandler(h func(http.ResponseWriter, *http.Request)) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		//httpcontext.Set(r, "params", ps)
		h(w, r)
	}
}
