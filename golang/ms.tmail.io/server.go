package main

import (
	//"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/codegangsta/negroni"
	"github.com/golang/protobuf/proto"
	"github.com/julienschmidt/httprouter"
	//"github.com/nbio/httpcontext"
)

var logger *log.Logger

// main launches HTTP server
func main() {
	out, err := os.OpenFile("/var/log/tmail/current.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln(err)
	}
	logger = log.New(out, "", log.Ldate|log.Lmicroseconds)

	router := httprouter.New()

	// Routes

	// Home
	router.GET("/", wrapHandler(hHome))

	// new smtpd client
	router.POST("/smtpdnewclient", wrapHandler(hNewSmtpdClient))

	// Server
	n := negroni.New(negroni.NewRecovery())
	n.UseHandler(router)
	log.Fatalln(http.ListenAndServe("127.0.0.1:3333", n))
}

//
func returnOnErr(err error, w http.ResponseWriter) bool {
	if err == nil {
		return false
	}
	logger.Printf("Error %s", err.Error())
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
	logger.Printf("%s - new /smtpdnewclient request", r.RemoteAddr)
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Printf("%s - Error %s", r.RemoteAddr, err.Error())
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
		logger.Printf("%s - suspicious IP %s", r.RemoteAddr, ipPort[0])
		isGrey, err := inGreyRbl(ipPort[0])
		if returnOnErr(err, w) {
			return
		}
		if isGrey {
			logger.Printf("%s - greylisted IP %s", r.RemoteAddr, ipPort[0])
			smtpResponse.SmtpCode = proto.Int32(421)
			smtpResponse.SmtpMsg = proto.String("i'm sorry Z, i'm afraid i can't let you do that now. try later.")
			smtpResponse.CloseConnection = proto.Bool(true)
		}
	}
	data, err = proto.Marshal(smtpResponse)
	if returnOnErr(err, w) {
		return
	}
	w.Write(data)
	logger.Printf("%s - ended", r.RemoteAddr)
}

// wrapHandler puts httprouter.Params in query context
// in order to keep compatibily with http.Handler
func wrapHandler(h func(http.ResponseWriter, *http.Request)) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		//httpcontext.Set(r, "params", ps)
		h(w, r)
	}
}
