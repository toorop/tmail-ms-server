package main

import (
	"io/ioutil"
	"log"
	"net/http"

	"github.com/golang/protobuf/proto"
)

// smtpdNewClientHandler handles /smtpdnewclient
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
	log.Println(err)
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(err.Error()))
	}

	// init response
	smtpResponse := &SmtpdResponse{
		SmtpCode: proto.Int32(220),
		SmtpMsg:  proto.String(""),
	}

	// test IP
	if newClientMsg.GetRemoteIp() == "127.0.0.1" {
		// return SMTP permFail
		smtpResponse.SmtpCode = proto.Int32(570)
		smtpResponse.SmtpMsg = proto.String("sorry you are not allowed to speak to me")
		// Drop connection
		smtpResponse.CloseConnection = proto.Bool(true)
	}
	data, err = proto.Marshal(smtpResponse)
	log.Println(err)
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
