package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/grindlemire/log"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
)

// NewGmailService authenticates to the gmail api and returns the service object
func NewGmailService() (service *gmail.Service, err error) {
	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		return service, errors.Wrap(err, "Unable to read client secret file")
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, gmail.GmailReadonlyScope)
	if err != nil {
		return service, errors.Wrap(err, "Unable to parse client secret file to config")
	}
	client, err := getClient(config)
	if err != nil {
		return service, errors.Wrap(err, "unable to get oauth client")
	}

	return gmail.New(client)
}

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) (client *http.Client, err error) {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok, err = getTokenFromWeb(config)
		if err != nil {
			return client, errors.Wrap(err, "unable to get auth token from web authentication")
		}
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok), nil
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) (tok *oauth2.Token, err error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the authorization code: \n%v\ncode: ", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return tok, errors.Wrap(err, "Unable to read authorization code")
	}

	tok, err = config.Exchange(context.TODO(), authCode)
	if err != nil {
		return tok, errors.Wrap(err, "Unable to retrieve token from web")
	}
	return tok, nil
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) (err error) {
	log.Infof("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errors.Wrap(err, "Unable to cache oauth token")
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(token)
}
