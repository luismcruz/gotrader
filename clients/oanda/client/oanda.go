package oandacl

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"net/http"
	"sync"
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

func (c *OandaClient) get(endpoint string) ([]byte, error) {

	url := c.restURL + endpoint

	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return nil, err
	}

	body, err := c.makeRequest(endpoint, req)

	if err != nil {
		return nil, err
	}

	return body, nil
}

func (c *OandaClient) put(endpoint string) ([]byte, error) {

	url := c.restURL + endpoint

	req, err := http.NewRequest(http.MethodPut, url, nil)

	if err != nil {
		return nil, err
	}

	body, err := c.makeRequest(endpoint, req)

	if err != nil {
		return nil, err
	}

	return body, nil
}

func (c *OandaClient) subscribe(endpoint string) (*bufio.Reader, error) {

	url := c.streamURL + endpoint

	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {
		return nil, err
	}

	c.setHeaders(req)

	res, err := c.streamClient.Do(req)

	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(res.Body)

	return reader, nil
}

func (c *OandaClient) post(endpoint string, data []byte) ([]byte, error) {

	url := c.restURL + endpoint

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(data))

	if err != nil {
		return nil, err
	}

	body, err := c.makeRequest(endpoint, req)

	if err != nil {
		return nil, err
	}

	return body, nil
}

func (c *OandaClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", c.headers.auth)
	req.Header.Set("Content-Type", c.headers.contentType)
}

func (c *OandaClient) makeRequest(endpoint string, req *http.Request) ([]byte, error) {

	c.setHeaders(req)

	res, err := c.restClient.Do(req)

	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return nil, err
	}

	return body, nil
}
