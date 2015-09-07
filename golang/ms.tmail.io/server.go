package main

import (
	//"fmt"
	"crypto/tls"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/codegangsta/negroni"
	"github.com/golang/protobuf/proto"
	"github.com/julienschmidt/httprouter"

	dkim "github.com/toorop/go-dkim"

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
	router.POST("/smtpdnewclient", wrapHandler(hSmtpdNewClient))
	router.POST("/smtpdnewclientgreysmtpd", wrapHandler(hSmtpdNewClient))

	// smtpdData
	router.POST("/smtpddata", wrapHandler(hSmtpdData))
	router.POST("/smtpddatadkimverif", wrapHandler(hSmtpdData))

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
func hSmtpdNewClient(w http.ResponseWriter, r *http.Request) {
	logger.Printf("%s - new /smtpdnewclient request", r.Header.Get("X-Real-IP"))
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Printf("%s - Error %s", r.RemoteAddr, err.Error())
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}
	newClientMsg := &SmtpdNewClientQuery{}
	err = proto.Unmarshal(data, newClientMsg)
	if returnOnErr(err, w) {
		return
	}

	response := &SmtpdNewClientResponse{
		SessionId: proto.String(newClientMsg.GetSessionId()),
		SmtpResponse: &SmtpResponse{
			Code: proto.Uint32(0),
			Msg:  proto.String(""),
		},
		DropConnection: proto.Bool(false),
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
			logger.Printf("%s - greylisted IP %s", r.Header.Get("X-Real-IP"), ipPort[0])
			response.SmtpResponse.Code = proto.Uint32(421)
			response.SmtpResponse.Msg = proto.String("i'm sorry Z, i'm afraid i can't let you do that now. try later.")
			response.DropConnection = proto.Bool(true)
		}
	}
	data, err = proto.Marshal(response)
	if returnOnErr(err, w) {
		return
	}
	w.Write(data)
	logger.Printf("%s - ended", r.Header.Get("X-Real-IP"))
}

// smtp DATA hook
func hSmtpdData(w http.ResponseWriter, r *http.Request) {
	logger.Printf("%s - new /smtpdData request", r.Header.Get("X-Real-IP"))
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Printf("%s - Error %s", r.RemoteAddr, err.Error())
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}
	query := &SmtpdDataQuery{}
	err = proto.Unmarshal(data, query)
	if returnOnErr(err, w) {
		return
	}

	// get raw message
	req, err := http.NewRequest("GET", query.GetDataLink(), nil)
	if returnOnErr(err, w) {
		return
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Timeout:   15 * time.Second,
		Transport: tr,
	}
	resp, err := client.Do(req)
	if returnOnErr(err, w) {
		return
	}
	defer resp.Body.Close()
	rawmail, err := ioutil.ReadAll(resp.Body)
	if returnOnErr(err, w) {
		return
	}

	smtpResponse := &SmtpdDataResponse{
		SessionId: proto.String(query.GetSessionId()),
		SmtpResponse: &SmtpResponse{
			Code: proto.Uint32(0),
			Msg:  proto.String(""),
		},
		ExtraHeaders:   []string{},
		DropConnection: proto.Bool(false),
	}

	flagHaveDkimHeader := true
	status, err := dkim.Verify(&rawmail)
	if err != nil && err == dkim.ErrDkimHeaderNotFound {
		flagHaveDkimHeader = false
	}

	header2add := "Authentication-Results: dkim="
	testing := false
	if !flagHaveDkimHeader {
		header2add += "pass (no DKIM header found)"
	} else {
		switch status {
		case dkim.PERMFAIL:
			header2add += "permfail "
		case dkim.TEMPFAIL:
			header2add += "tempfail "
		case dkim.SUCCESS:
			header2add += "success"
		case dkim.TESTINGPERMFAIL, dkim.TESTINGTEMPFAIL, dkim.TESTINGSUCCESS:
			testing = true
			header2add += "success testing "
		}
		if err != nil && !testing {
			header2add += err.Error()
		}
	}

	smtpResponse.ExtraHeaders = append(smtpResponse.ExtraHeaders, header2add)

	data, err = proto.Marshal(smtpResponse)
	if returnOnErr(err, w) {
		return
	}
	w.Write(data)
	logger.Printf("%s - ended", r.Header.Get("X-Real-IP"))

}

// wrapHandler puts httprouter.Params in query context
// in order to keep compatibily with http.Handler
func wrapHandler(h func(http.ResponseWriter, *http.Request)) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		//httpcontext.Set(r, "params", ps)
		h(w, r)
	}
}
