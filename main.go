package main

import (
	"io"
	"syscall"

	"github.com/grindlemire/gmail-sync/pkg/auth"
	"github.com/grindlemire/gmail-sync/pkg/db"
	"github.com/grindlemire/gmail-sync/pkg/gmail"
	"github.com/grindlemire/log"
	"github.com/olivere/elastic/v7"
	"github.com/vrecan/death"
)

const user = "me"

func main() {
	log.Init(log.Default)

	client, err := auth.NewGmailService()
	if err != nil {
		log.Fatalf("unable to access gmail api: %v", err)
	}

	esClient, err := elastic.NewSimpleClient(
		elastic.SetURL("http://127.0.0.1:9200"),
	)
	if err != nil {
		log.Fatalf("unable to create elasticsearch client: %v", err)
	}

	d := death.NewDeath(syscall.SIGINT, syscall.SIGTERM)
	goRoutines := []io.Closer{}

	flusher := db.NewESFlusher(esClient)
	flusher.Start()
	goRoutines = append(goRoutines, flusher)

	processor := gmail.NewMessageProcessor(client, flusher, d)
	processor.Start()
	goRoutines = append(goRoutines, processor)

	d.WaitForDeath(goRoutines...)
}
