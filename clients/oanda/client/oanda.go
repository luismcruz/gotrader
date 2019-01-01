package oandacl

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

type Headers struct {
	contentType string
	auth        string
}

type OandaClient struct {
	restURL                  string
	streamURL                string
	token                    string
	headers                  Headers
	restClient               http.Client
	streamClient             http.Client
	priceSubscriptions       map[string]map[string]bool
	transactionSubscriptions map[string]*transactionTypeLogic
	mutex                    sync.Locker
}

func NewClient(token string, live bool) *OandaClient {

	var (
		restURL   string
		streamURL string
	)

	if live {
		restURL = "https://api-fxtrade.oanda.com/v3"
		streamURL = "https://stream-fxtrade.oanda.com/v3"
	} else {
		restURL = "https://api-fxpractice.oanda.com/v3"
		streamURL = "https://stream-fxpractice.oanda.com/v3"
	}

	headers := Headers{
		contentType: "application/json",
		auth:        "Bearer " + token,
	}

	// Create the connection object
	connection := &OandaClient{
		token:                    token,
		restURL:                  restURL,
		streamURL:                streamURL,
		headers:                  headers,
		restClient:               http.Client{},
		streamClient:             http.Client{},
		priceSubscriptions:       make(map[string]map[string]bool),
		transactionSubscriptions: make(map[string]*transactionTypeLogic),
		mutex: &sync.Mutex{},
	}

	return connection
}

func (c *OandaClient) get(endpoint string) []byte {

	url := c.restURL + endpoint

	req, err := http.NewRequest(http.MethodGet, url, nil)
	checkErr(err)

	body := c.makeRequest(endpoint, req)

	return body
}

func (c *OandaClient) put(endpoint string) []byte {

	url := c.restURL + endpoint

	req, err := http.NewRequest(http.MethodPut, url, nil)
	checkErr(err)

	body := c.makeRequest(endpoint, req)

	return body
}

func (c *OandaClient) subscribe(endpoint string) *bufio.Reader {

	url := c.streamURL + endpoint

	req, err := http.NewRequest(http.MethodGet, url, nil)
	checkErr(err)

	c.setHeaders(req)

	res, getErr := c.streamClient.Do(req)
	checkErr(getErr)

	reader := bufio.NewReader(res.Body)

	return reader
}

func (c *OandaClient) post(endpoint string, data []byte) []byte {

	url := c.restURL + endpoint

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))
	checkErr(err)

	body := c.makeRequest(endpoint, req)

	return body
}

func (c *OandaClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", c.headers.auth)
	req.Header.Set("Content-Type", c.headers.contentType)
}

func (c *OandaClient) makeRequest(endpoint string, req *http.Request) []byte {
	c.setHeaders(req)

	res, getErr := c.restClient.Do(req)
	checkErr(getErr)
	body, readErr := ioutil.ReadAll(res.Body)
	checkErr(readErr)
	checkAPIErr(body, endpoint)

	return body
}

func unmarshalJSON(body []byte, data interface{}) {
	jsonErr := json.Unmarshal(body, &data)
	checkErr(jsonErr)
}

func checkErr(err error) {
	if err != nil {
		log.Warning(err)
	}
}

func checkAPIErr(body []byte, route string) {

	bodyString := string(body[:])

	if strings.Contains(bodyString, "errorMessage") {
		log.Warning("\nOANDA API Error: " + bodyString + "\nOn route: " + route)
	}

}
