package oandacl

import (
	"net/url"
	"strings"
	"time"
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

func (c *OandaClient) GetPrices(accountID string, instruments []string) Pricings {

	instrumentString := strings.Join(instruments, ",")
	endpoint := "/accounts/" + accountID + "/pricing?instruments=" + url.QueryEscape(instrumentString)

	response := c.get(endpoint)
	data := Pricings{}
	unmarshalJSON(response, &data)
	return data
}

func (c *OandaClient) SubscribePrices(accountID string, instruments []string, handler PriceHandler) {

	if c.managePriceSubscriptions(accountID, instruments) {
		c.subscribePrices(accountID, handler)
	}

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

func (c *OandaClient) subscribePrices(accountID string, handler PriceHandler) {

	go func() {

		instrumentString := strings.Join(c.getInstrumentsList(accountID), ",")
		endpoint := "/accounts/" + accountID + "/pricing/stream?instruments=" + url.QueryEscape(instrumentString)

		reader := c.subscribe(endpoint)

		for {

			line, err := reader.ReadBytes('\n')
			checkErr(err)

			if strings.Contains(string(line), "\"type\":\"HEARTBEAT\"") {
				continue
			}

			data := Price{}
			unmarshalJSON(line, &data)
			handler(data)

		}

	}()

}
