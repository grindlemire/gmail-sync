# gmail-sync

Use the gmail api to export your emails into elasticsearch for visualization by kibana.

This is a modernized version of [this medium article](https://medium.com/@orweinberger/analyze-and-visualize-your-gmail-inbox-using-elasticsearch-and-kibana-88cb4e373c13) written in golang and using elasticsearch 7.x

## To run:

1. Ensure you have elasticsearch and kibana 7.x running locally
1. Ensure you have [golang](https://golang.org/) installed
1. run
   ```
   git clone https://github.com/grindlemire/gmail-sync
   ```
1. Ensure you have the gmail api turned on for the email you want to sync. See [this article](https://developers.google.com/gmail/api/quickstart/go) for how to enable the api. Make sure the `credentials.json` file is placed in the gmail-sync directory
1. run `go run main.go` in the gmail-sync directory

## Note

Depending on how many emails you have it may burn through your allowed quota and you will not be able to sync any more emails.

I log out the current page token that is being used (it keeps track of which page of results you are looking at) so you can start at that token when you next have quota available. To start at a specific page (rather than the beginning of your messages) simply specify the token via `-t` command line flag. Example:

```
./gmail-sync -t abcdefhij
```
