package oandacl

import (
	"bufio"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type Pricings struct {
	Prices []*Price `json:"prices"`
}

type Price struct {
	Asks        []PriceBucket `json:"asks"`
	Bids        []PriceBucket `json:"bids"`
	CloseoutAsk string        `json:"closeoutAsk"`
	CloseoutBid string        `json:"closeoutBid"`
	Instrument  string        `json:"instrument"`
	Time        time.Time     `json:"time"`
}

type PriceBucket struct {
	Liquidity int     `json:"liquidity"`
	Price     float64 `json:"price,string"`
}

type PriceHandler func(price Price)

func (c *OandaClient) GetPrices(accountID string, instruments []string) (Pricings, error) {

	instrumentString := strings.Join(instruments, ",")
	endpoint := "/accounts/" + accountID + "/pricing?instruments=" + url.QueryEscape(instrumentString)

	response, err := c.get(endpoint)

	if err != nil {
		return Pricings{}, err
	}

	data := Pricings{}
	err = json.Unmarshal(response, &data)

	if err != nil {
		return Pricings{}, err
	}

	return data, nil
}

func (c *OandaClient) SubscribePrices(accountID string, instruments []string, handler PriceHandler) error {

	if c.managePriceSubscriptions(accountID, instruments) {
		err := c.subscribePrices(accountID, handler)

		if err != nil {
			return err
		}
	}

	return nil
}

func (c *OandaClient) managePriceSubscriptions(accountID string, instruments []string) bool {

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exist := c.priceSubscriptions[accountID]; !exist {
		c.priceSubscriptions[accountID] = make(map[string]bool)
	}

	subscribe := false

	for _, inst := range instruments {
		if val, exist := c.priceSubscriptions[accountID][inst]; !exist || !val {
			c.priceSubscriptions[accountID][inst] = true
			subscribe = true
		}
	}

	return subscribe

}

func (c *OandaClient) getInstrumentsList(accountID string) []string {

	var length int

	if _, exist := c.priceSubscriptions[accountID]; exist {
		length = len(c.priceSubscriptions[accountID])
	}

	instrumentsList := make([]string, 0, length)

	for inst := range c.priceSubscriptions[accountID] {
		if c.priceSubscriptions[accountID][inst] {
			instrumentsList = append(instrumentsList, inst)
		}
	}

	return instrumentsList

}

func (c *OandaClient) subscribePrices(accountID string, handler PriceHandler) error {

	instrumentString := strings.Join(c.getInstrumentsList(accountID), ",")
	endpoint := "/accounts/" + accountID + "/pricing/stream?instruments=" + url.QueryEscape(instrumentString)

	reader, err := c.subscribe(endpoint)

	if err != nil {
		return err
	}

	go func(reader *bufio.Reader) {

		for {

			line, err := reader.ReadBytes('\n')

			if err != nil {
				logrus.Warn(err)
				continue
			}

			if strings.Contains(string(line), "\"type\":\"HEARTBEAT\"") {
				continue
			}

			data := Price{}
			err = json.Unmarshal(line, &data)

			if err != nil {
				logrus.Warn(err)
				continue
			}

			handler(data)
		}

	}(reader)

	return nil
}
