package main

import (
	"flag"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/caddyserver/certmagic"
	"github.com/eze-kiel/masker/handlers"
)

func main() {
	var prod bool
	flag.BoolVar(&prod, "prod", false, "production mode")
	flag.Parse()
	switch prod {
	case true:
		// read and agree to your CA's legal documents
		certmagic.DefaultACME.Agreed = true

		// provide an email address
		certmagic.DefaultACME.Email = "hugoblanc@fastmail.com"

		log.Info("[PROD] Server is starting, wish me luck boys")
		certmagic.HTTPS([]string{"masker.freeboard.tech"}, handlers.Handle())

	case false:
		srv := &http.Server{
			Addr:         ":8080",
			Handler:      handlers.Handle(),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		log.Info("[DEV] Server is starting, wish me luck boys")
		log.Println(srv.ListenAndServe())
	}
}
