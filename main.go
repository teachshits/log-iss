package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	forwarder := NewForwarder()
	forwarder.Start()

	tokens, err := parseTokens()
	if err != nil {
		log.Fatalln("Unable to parse tokens:", err)
	}

	http.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Only POST is accepted", 400)
			return
		}
		if r.Header.Get("Content-Type") != "application/logplex-1" {
			http.Error(w, "Only Content-Type application/logplex-1 is accepted", 400)
			return
		}

		err := checkAuth(r, tokens)
		if err != nil {
			http.Error(w, err.Error(), 401)
		}

		b, err := ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			http.Error(w, "Invalid Request", 400)
		}

		forwarder.Receive(b)
	})

	if err := http.ListenAndServe(":5000", nil); err != nil {
		log.Fatalln("Unable to start HTTP server:", err)
	}
}

func parseTokens() (map[string]string, error) {
	tokens := make(map[string]string)

	tokenMap := os.Getenv("TOKEN_MAP")
	if tokenMap == "" {
		return tokens, errors.New("ENV[TOKEN_MAP] is required")
	}

	for _, userAndToken := range strings.Split(tokenMap, ",") {
		userAndTokenParts := strings.SplitN(userAndToken, ":", 2)
		if len(userAndTokenParts) != 2 {
			return tokens, errors.New("ENV[TOKEN_MAP] not formatted properly")
		}
		tokens[userAndTokenParts[0]] = userAndTokenParts[1]
	}

	return tokens, nil
}

func checkAuth(r *http.Request, tokens map[string]string) error {
	header := r.Header.Get("Authorization")
	if header == "" {
		return errors.New("Authorization required")
	}
	headerParts := strings.SplitN(header, " ", 2)
	if len(headerParts) != 2 {
		return errors.New("Authorization header is malformed")
	}

	method := headerParts[0]
	if method != "Basic" {
		return errors.New("Only Basic Authorization is accepted")
	}

	encodedUserPass := headerParts[1]
	decodedUserPass, err := base64.StdEncoding.DecodeString(encodedUserPass)
	if err != nil {
		return errors.New("Authorization header is malformed")
	}

	userPassParts := bytes.SplitN(decodedUserPass, []byte{':'}, 2)
	if len(userPassParts) != 2 {
		return errors.New("Authorization header is malformed")
	}

	user := userPassParts[0]
	pass := userPassParts[1]
	token, ok := tokens[string(user)]
	if !ok {
		return errors.New("Unknown user")
	}
	if token != string(pass) {
		return errors.New("Incorrect token")
	}

	return nil
}

type Forwarder struct {
	Inbox   chan []byte
	c       net.Conn
	written uint64
}

func NewForwarder() *Forwarder {
	forwarder := new(Forwarder)
	forwarder.Inbox = make(chan []byte, 25*1024*1024)
	return forwarder
}

func (f *Forwarder) Receive(b []byte) {
	f.Inbox <- b
}

func (f *Forwarder) Start() {
	go f.Run()
	go f.PeriodicStats()
}

func (f *Forwarder) Run() {
	for m := range f.Inbox {
		f.write(m)
	}
}

func (f *Forwarder) PeriodicStats() {
	var connected string
	t := time.Tick(1 * time.Second)

	for {
		<-t
		connected = "no"
		if f.c != nil {
			connected = "yes"
		}
		log.Printf("ns=forwarder fn=periodic_stats at=emit connected=%s written=%d\n", connected, f.written)
	}
}

func (f *Forwarder) connect() {
	if f.c != nil {
		return
	}

	rate := time.Tick(200 * time.Millisecond)

	for {
		log.Println("ns=forwarder fn=connect at=start")
		c, err := net.Dial("tcp", "localhost:5001")
		if err != nil {
			log.Printf("ns=forwarder fn=connect at=error message=%q\n", err)
			f.disconnect()
		} else {
			log.Println("ns=forwarder fn=connect at=finish")
			f.c = c
			return
		}
		<-rate
	}
}

func (f *Forwarder) disconnect() {
	if f.c != nil {
		f.c.Close()
	}
	f.c = nil
}

func (f *Forwarder) write(b []byte) {
	for {
		f.connect()
		n, err := f.c.Write(b)
		if err != nil {
			log.Println("ns=forwarder fn=write at=error message=%q\n", err)
			f.disconnect()
		} else {
			f.written += uint64(n)
			break
		}
	}
}