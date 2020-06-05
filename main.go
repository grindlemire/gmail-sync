package main

import (
	"io"
	"os"
	"syscall"

	"github.com/grindlemire/gmail-sync/pkg/auth"
	"github.com/grindlemire/gmail-sync/pkg/db"
	"github.com/grindlemire/gmail-sync/pkg/gmail"
	"github.com/grindlemire/log"
	"github.com/jessevdk/go-flags"
	"github.com/olivere/elastic/v7"
	"github.com/vrecan/death"
)

// Opts ...
type Opts struct {
	PageToken string `short:"t" long:"page-token" default:"" description:"the token to start with if you want to run at an offset"`
}

var opts Opts
var parser = flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash)

func main() {
	log.Init(log.Default)

	_, err := parser.Parse()
	if flags.WroteHelp(err) {
		parser.WriteHelp(os.Stderr) // This writes the help when we want help. This is silenced because we are not writing any errors
		os.Exit(1)
	}
	if err != nil {
		log.Fatal(err)
	}

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

	processor := gmail.NewMessageProcessor(client, flusher, d, opts.PageToken)
	processor.Start()
	goRoutines = append(goRoutines, processor)

	d.WaitForDeath(goRoutines...)
}
