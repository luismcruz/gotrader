package oandacl

import (
	"bufio"
	"encoding/json"
	"math"
	"net/url"
	"strings"
	"sync"
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

func (c *OandaClient) SubscribePrices(accountID string, instruments []string, handler PriceHandler) (*PriceSubscription, error) {

	subscription := newPriceSubscrption(c.dial, handler, accountID)
	err := subscription.subscribe(instruments)

	if err != nil {
		return nil, err
	}

	return subscription, nil
}

// PriceSubscription represents a subscription, can be used to free resources
type PriceSubscription struct {
	priceSubscriptions   map[string]bool
	stopPriceSubscripton chan bool
	handler              PriceHandler
	mutex                *sync.Mutex
	dial                 func(endpoint string) (*bufio.Reader, error)
	accountID            string
	activeSubscription   bool
}

func newPriceSubscrption(dial func(endpoint string) (*bufio.Reader, error),
	handler PriceHandler, accountID string) *PriceSubscription {

	return &PriceSubscription{
		priceSubscriptions:   make(map[string]bool),
		stopPriceSubscripton: make(chan bool),
		mutex:                &sync.Mutex{},
		dial:                 dial,
		accountID:            accountID,
		handler:              handler,
	}
}

// Stop will stop the subscription
func (s *PriceSubscription) Stop() {
	s.stopPriceSubscripton <- true
}

func (s *PriceSubscription) subscribe(instruments []string) error {

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.needsToDial(instruments) {
		if err := s.subscribePrices(); err != nil {
			return err
		}
	}

	return nil
}

func (s *PriceSubscription) subscribePrices() error {

	instrumentString := strings.Join(s.getInstrumentsList(s.accountID), ",")
	endpoint := "/accounts/" + s.accountID + "/pricing/stream?instruments=" + url.QueryEscape(instrumentString)

	reader, err := s.dial(endpoint)

	if err != nil {
		return err
	}

	if s.activeSubscription { // Shuts down previous go routine
		s.stopPriceSubscripton <- true // channel is not buffered to make sure it waits for the shutdown
	} else {
		s.activeSubscription = true
	}

	go func(reader *bufio.Reader, endpoint string) {

	subLoop:
		for {
			select {
			case <-s.stopPriceSubscripton:
				break subLoop
			default:

				line, err := reader.ReadBytes('\n')

				if err != nil {

					logrus.Warn(err)

					if reader, err = s.reconnect(endpoint); err != nil { // Did not recover subscription, break outer loop
						break subLoop
					}

					continue
				}

				if strings.Contains(string(line), "\"type\":\"HEARTBEAT\"") { // Ignore heartbeats
					continue
				}

				data := Price{}
				err = json.Unmarshal(line, &data)

				if err != nil {
					logrus.Warn(err)
					continue
				}

				s.handler(data)
			}
		}

		s.activeSubscription = false

	}(reader, endpoint)

	return nil
}

func (s *PriceSubscription) reconnect(endpoint string) (reader *bufio.Reader, err error) {

	for i := 0; i < 3; i++ { // Try reconnection 3 times with exponential backoff

		logrus.Info("Trying to recover subscription...")

		time.Sleep(time.Duration(math.Pow(2, float64(i))) * 100 * time.Millisecond)

		reader, err = s.dial(endpoint)

		if err == nil {
			logrus.Info("Subscription recovered")
			return
		}
	}

	return
}

func (s *PriceSubscription) needsToDial(instruments []string) (subscribe bool) {

	for _, inst := range instruments {
		if !s.priceSubscriptions[inst] {
			s.priceSubscriptions[inst] = true
			subscribe = true
		}
	}

	return
}

func (s *PriceSubscription) getInstrumentsList(accountID string) []string {

	instrumentsList := make([]string, 0, len(s.priceSubscriptions))
	for inst := range s.priceSubscriptions {
		instrumentsList = append(instrumentsList, inst)
	}

	return instrumentsList

}
