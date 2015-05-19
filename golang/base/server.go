package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/golang/protobuf/proto"
)

// smtpdNewClientHandler handles /smtpdnewclient
// If smtpd client IP is 127.0.0.1 connection is rejected
func smtpdNewClientHandler(w http.ResponseWriter, r *http.Request) {
	// read request body, ie a protobuf serialized SmtpdNewClientMsg
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		// for the poc simply return HTTP 500 on error
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}

	newClientMsg := &SmtpdNewClientMsg{}
	err = proto.Unmarshal(data, newClientMsg)
	if err != nil {
		w.WriteHeader(422)
		w.Write([]byte(err.Error()))
		return
	}

	// init response
	smtpResponse := &SmtpdResponse{
		SmtpCode: proto.Int32(0),
		SmtpMsg:  proto.String(""),
	}

	// test client IP (ip:port)
	ipPort := strings.Split(newClientMsg.GetRemoteIp(), ":")
	//  bad format ?
	if len(ipPort) != 2 {
		w.WriteHeader(422)
		w.Write([]byte("422 - Bad remote IP format. Expected ip:port. Got: " + newClientMsg.GetRemoteIp()))
		return
	}

	if ipPort[0] == "127.0.0.1" {
		// return SMTP permFail
		smtpResponse.SmtpCode = proto.Int32(570)
		smtpResponse.SmtpMsg = proto.String("sorry you are not allowed to speak to me")
		// Close connection
		smtpResponse.CloseConnection = proto.Bool(true)
	}
	data, err = proto.Marshal(smtpResponse)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}
	w.Write(data)
}

func main() {
	http.HandleFunc("/smtpdnewclient", smtpdNewClientHandler)
	log.Fatalln(http.ListenAndServe(":3333", nil))
}
