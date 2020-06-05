package db

import (
	"context"
	"crypto/md5"
	"fmt"
	"time"

	"github.com/grindlemire/log"
	"github.com/olivere/elastic/v7"
	"github.com/pkg/errors"
	"github.com/vrecan/life"
)

// Document is a document bound for Elasticsearch
type Document struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Date      time.Time `json:"date"`
	Subject   string    `json:"subject"`
	HourOfDay int       `json:"hourOfDay"`
	DayOfWeek string    `json:"dayOfWeek"`
	Secure    bool      `json:"secure"`
}

// ID gets the id of the document
func (d Document) ID() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%s|%s|%s|%s", d.From, d.To, d.Date.Format(time.RFC3339), d.Subject))))
}

const batchSize = 20

// Flusher flushes docuuments to ES
type Flusher struct {
	*life.Life

	input chan Document
	bs    *elastic.BulkService
	total int
}

// NewESFlusher creates a new flusher
func NewESFlusher(client *elastic.Client) (f *Flusher) {
	f = &Flusher{
		input: make(chan Document, 100),
		Life:  life.NewLife(),
		bs:    elastic.NewBulkService(client).Index("gmail"),
		total: 0,
	}
	f.SetRun(f.run)
	return f
}

func (f *Flusher) run() {
	for {
		select {
		case <-f.Done:
			err := f.drain()
			if err != nil {
				log.Errorf("error while draining internal buffers: %v", err)
			}
			return
		case doc := <-f.input:
			f.add(doc)
		}
	}
}

// Close down the flusher
func (f *Flusher) Close() error {
	log.Info("shutting down flusher")
	err := f.Life.Close()
	log.Info("successfully shutdown flusher")
	return err
}

func (f *Flusher) drain() error {
	for {
		select {
		case doc := <-f.input:
			err := f.add(doc)
			if err != nil {
				log.Errorf("error adding document to bulk rquest while preparing for final flush: %v", err)
			}
		default:
			_, err := f.flush()
			if err != nil {
				log.Error("error flushing flusher for final time: %v", err)
			}
			return err
		}
	}
}

// Add a document to the flusher
func (f *Flusher) Add(doc Document) (err error) {
	select {
	case f.input <- doc:
		return nil
	default:
		return errors.Errorf("unable to load message on flusher. Internall buffer full.")
	}
}

func (f *Flusher) add(doc Document) (err error) {
	req := elastic.NewBulkIndexRequest().Id(doc.ID()).Doc(doc)
	f.bs.Add(req)

	if f.bs.NumberOfActions()%10 == 0 {
		log.Infof("Batch contains %v messages", f.bs.NumberOfActions())
	}
	if f.bs.NumberOfActions() >= batchSize {
		_, err = f.flush()
		if err != nil {
			return errors.Wrap(err, "failed to flush to elasticsearch")
		}
	}

	return nil
}

// flush the documents to Elasticsearch
func (f *Flusher) flush() (n int, err error) {
	if f.bs.NumberOfActions() == 0 {
		return 0, nil
	}

	incoming := f.bs.NumberOfActions()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := f.bs.Do(ctx)
	if err != nil {
		return 0, err
	}

	succeeded := len(resp.Succeeded())
	if succeeded < incoming {
		if len(resp.Failed()) > 0 {
			log.Errorf("Error inserting to ES: %v", resp.Failed()[0].Error.Reason)
		}
		return 0, errors.Errorf("Number of succcessful inserts less than inserted: %v", succeeded)
	}

	f.total += succeeded
	log.Infof("Total flushed: %v", f.total)

	f.bs.Reset()
	return succeeded, nil
}
