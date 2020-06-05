package gmail

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/grindlemire/gmail-sync/pkg/db"
	"github.com/grindlemire/log"
	"github.com/pkg/errors"
	"github.com/vrecan/death"
	"github.com/vrecan/life"
	"google.golang.org/api/gmail/v1"
)

// user is the user to query for
const user = "me"

// timeFormats are the supported formats for parsing time from email
var timeFomats = []string{
	time.RFC1123,
	time.RFC1123Z,
	fmt.Sprintf("Mon, 02 Jan 2006 15:04:05 -0700 (MST)"),
	fmt.Sprintf("Mon, 02 Jan 2006 15:04:05 -0700"),
	fmt.Sprintf("Mon, 2 Jan 2006 15:04:05 -0700 (MST)"),
	fmt.Sprintf("Mon, 2 Jan 2006 15:04:05 -0700"),
	fmt.Sprintf("02 Jan 2006 15:04:05 -0700"),
	fmt.Sprintf("2 Jan 2006 15:04:05 -0700"),
}

// Processor processes gmail messages via the API
type Processor struct {
	*life.Life
	client  *gmail.Service
	flusher *db.Flusher
	d       *death.Death
}

// NewMessageProcessor creates a new gmail message processor
func NewMessageProcessor(client *gmail.Service, flusher *db.Flusher, d *death.Death) (p *Processor) {
	p = &Processor{
		Life:    life.NewLife(),
		client:  client,
		flusher: flusher,
		d:       d,
	}
	p.Life.SetRun(p.run)
	return p
}

func (p *Processor) run() {
	p.processMessages()
	for {
		select {
		case <-p.Life.Done:
			return
		}
	}
}

func (p *Processor) processMessages() (err error) {
	pageToken := ""
	for {
		select {
		case <-p.Life.Done:
			return
		default:
		}

		r, err := p.client.Users.Messages.List(user).PageToken(pageToken).Do()
		if err != nil {
			return err
		}

		err = p.processMessageBatch(r.Messages)
		if err != nil {
			return err
		}

		pageToken = r.NextPageToken
		if pageToken == "" {
			p.d.FallOnSword()
			return nil
		}
	}
}

func (p *Processor) processMessageBatch(batch []*gmail.Message) error {
	wg := &sync.WaitGroup{}
	for _, m := range batch {
		select {
		case <-p.Life.Done:
			return nil
		default:
		}

		go func(m *gmail.Message) {
			wg.Add(1)
			defer wg.Done()
			message, err := p.client.Users.Messages.Get(user, m.Id).Do()
			if err != nil {
				log.Fatalf("unable to retrieve raw message for message %v: %v", m.Id, err)
			}

			errFound := false
			from, err := getHeader("from", message.Payload.Headers)
			if err != nil {
				log.Warnf("unable to parse from header for message %v: %v", m.Id, err)
				errFound = true
			}
			to, err := getHeader("to", message.Payload.Headers)
			if err != nil {
				log.Warnf("unable to parse to header for message %v: %v", m.Id, err)
				errFound = true
			}
			rawDate, err := getHeader("date", message.Payload.Headers)
			if err != nil {
				log.Warnf("unable to get date header for message %v: %v", m.Id, err)
				errFound = true
			}
			date, err := parseDate(rawDate)
			if err != nil {
				log.Warnf("unable to parse date header for message %v: %v", m.Id, err)
				errFound = true
			}

			subject, err := getHeader("subject", message.Payload.Headers)
			if err != nil {
				log.Warnf("unable to parse subject header for message %v: %v", m.Id, err)
				errFound = true
			}

			secure := checkSecure(message.Payload.Headers)

			doc := db.Document{
				From:      from,
				To:        to,
				Date:      date,
				Subject:   subject,
				HourOfDay: date.Hour(),
				DayOfWeek: date.Weekday().String(),
				Secure:    secure,
			}

			if errFound {
				log.Warnf("Information we know on document that failed full parsing (skipping inserting it): %+v", doc)
				return
			}

			err = p.flusher.Add(doc)
			if err != nil {
				log.Errorf("unable to add message to batch: %v", err)
				return
				// return errors.Wrap(err, "failed to add message to batch")
			}
		}(m)
	}
	wg.Wait()
	return nil
}

// Close initiates a shutdown of the processor
func (p *Processor) Close() error {
	log.Info("shutting down for processor")
	err := p.Life.Close()
	log.Info("successfully shutdown processor")
	return err
}

func checkSecure(headers []*gmail.MessagePartHeader) (secure bool) {
	spfFound := false
	dmarcFound := false
	dkimFound := false
	for _, header := range headers {
		if strings.Contains(strings.ToLower(header.Value), "spf=pass") {
			spfFound = true
		}

		if strings.Contains(strings.ToLower(header.Value), "dkim=pass") {
			dkimFound = true
		}

		if strings.Contains(strings.ToLower(header.Value), "dmarc=pass") {
			dmarcFound = true
		}
	}

	return spfFound || dmarcFound || dkimFound
}

func parseDate(rawDate string) (date time.Time, err error) {
	for _, format := range timeFomats {
		date, err = time.Parse(format, rawDate)
		if err == nil {
			return date, nil
		}
	}
	return date, err
}

func getHeader(name string, headers []*gmail.MessagePartHeader) (s string, err error) {
	for _, header := range headers {
		if strings.EqualFold(header.Name, name) {
			return header.Value, nil
		}
	}
	return "", errors.Errorf("header [%v] not found", name)
}
