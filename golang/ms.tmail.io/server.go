package main

import (
	//"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

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

	// Server
	n := negroni.New(negroni.NewRecovery(), negroni.NewLogger())
	n.UseHandler(router)
	log.Fatalln(http.ListenAndServe("127.0.0.1:3333", n))
}

//
func returnOnErr(err error, w http.ResponseWriter) bool {
	if err == nil {
		return false
	}
	w.WriteHeader(500)
	w.Write([]byte(err.Error()))
	return true
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
	if returnOnErr(err, w) {
		return
	}

	smtpResponse := &SmtpdResponse{
		SmtpCode: proto.Int32(220),
		SmtpMsg:  proto.String(""),
	}

	ipPort := strings.Split(newClientMsg.GetRemoteIp(), ":")
	if len(ipPort) != 2 {
		w.WriteHeader(422)
		w.Write([]byte("422 - Bad remote IP format. Expected ip:port. Got: " + newClientMsg.GetRemoteIp()))
		return
	}

	// blacklisted IP
	//ipPort[0] = "85.70.31.200"
	suspicious, err := ipHaveNoReverse(ipPort[0])
	if returnOnErr(err, w) {
		return
	}

	if !suspicious {
		suspicious, err = isBlacklistedOn(ipPort[0], "bl.spamcop.net")
		if returnOnErr(err, w) {
			return
		}
	}

	if suspicious {
		isGrey, err := inGreyRbl(ipPort[0])
		if returnOnErr(err, w) {
			return
		}
		if isGrey {
			smtpResponse.SmtpCode = proto.Int32(421)
			smtpResponse.SmtpMsg = proto.String("suspicious IP, try later")
			smtpResponse.CloseConnection = proto.Bool(true)
		}
	}
	data, err = proto.Marshal(smtpResponse)
	if returnOnErr(err, w) {
		return
	}
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
