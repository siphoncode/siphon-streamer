package streamer

import (
	"log"
	"net/http"
	"os"
)

// Start the streamer
func Start() {
	log.Printf("Starting up...")
	go d.run() // Start the dispatcher
	go runNotificationListener()

	http.HandleFunc("/v1/streams/", websocketHandler)

	if os.Getenv("SIPHON_ENV") == "testing" {
		http.ListenAndServe(":8080", nil)
	} else {
		certFile := "/code/.keys/getsiphon-com-bundle.crt"
		keyFile := "/code/.keys/host.pem"
		http.ListenAndServeTLS(":443", certFile, keyFile, nil)
		log.Fatal(http.ListenAndServeTLS(":443", certFile, keyFile, nil))
		log.Print("Listening on 443...")
	}
}
