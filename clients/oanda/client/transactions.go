package oandacl

import (
	"bufio"
	"encoding/json"
	"math"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/uber-go/atomic"
)

type TradeFinancing struct {
	TradeID   string  `json:"tradeID"`
	Financing float64 `json:"financing,string"`
}

type PositionFinancing struct {
	Instrument      string            `json:"instrument"`
	Financing       float64           `json:"financing,string"`
	TradesFinancing []*TradeFinancing `json:"openTradeFinancings"`
}

type Transaction struct {
	AccountBalance     float64              `json:"accountBalance,string"`
	AccountID          string               `json:"accountID"`
	Amount             float64              `json:"amount,string"`
	Financing          float64              `json:"financing,string"`
	ID                 string               `json:"id"`
	Instrument         string               `json:"instrument"`
	OrderID            string               `json:"orderID"`
	Pl                 float64              `json:"pl,string"`
	Price              float64              `json:"price,string"`
	Reason             string               `json:"reason"`
	Time               time.Time            `json:"time"`
	TradeOpened        *TradeOpened         `json:"tradeOpened"`
	TradesClosed       []*TradeReduced      `json:"tradesClosed"`
	PositionFinancings []*PositionFinancing `json:"positionFinancings"`
	Type               string               `json:"type"`
	Units              string               `json:"units"`
	RejectReason       *string              `json:"rejectReason"`
}

type TransactionHandler func(transaction *Transaction)

type transactionTypeLogic struct {
	orderFill     *atomic.Bool
	orderCreate   *atomic.Bool
	orderCancel   *atomic.Bool
	financing     *atomic.Bool
	fundsTransfer *atomic.Bool
}

func newTransactionTypeLogic() *transactionTypeLogic {
	return &transactionTypeLogic{
		orderFill:     atomic.NewBool(false),
		orderCreate:   atomic.NewBool(false),
		orderCancel:   atomic.NewBool(false),
		financing:     atomic.NewBool(false),
		fundsTransfer: atomic.NewBool(false),
	}
}

func (t *transactionTypeLogic) hydrate(transType []TransactionType) {

	if transType == nil || len(transType) == 0 {
		t.orderCreate.Toggle()
		t.orderFill.Toggle()
		t.orderCreate.Toggle()
		t.financing.Toggle()
		t.fundsTransfer.Toggle()
	} else {
		for _, tr := range transType {
			switch tr {
			case OrderCreate:
				t.orderCreate.Toggle()
			case OrderFill:
				t.orderFill.Toggle()
			case OrderCancel:
				t.orderCreate.Toggle()
			case Financing:
				t.financing.Toggle()
			case FundsTransfer:
				t.fundsTransfer.Toggle()
			}
		}
	}

}

func (t *transactionTypeLogic) checkIgnore(transaction *Transaction) bool {

	if transaction.Type == "ORDER_FILL" || transaction.Type == "MARKET_ORDER_REJECT" { // Order Fill

		transaction.Type = "ORDER_FILL"
		return !t.orderFill.Load()

	} else if transaction.Type == "LIMIT_ORDER" || transaction.Type == "STOP_ORDER" ||
		transaction.Type == "MARKET_IF_TOUCHED_ORDER" { // Order Create

		return !t.orderCreate.Load()

	} else if transaction.Type == "TRANSFER_FUNDS" { // Funds Transfer

		return !t.fundsTransfer.Load()

	} else if transaction.Type == "DAILY_FINANCING" { // Daily Financing

		return !t.financing.Load()
	}

	return true

}

type TransactionType int

const (
	OrderFill TransactionType = iota
	OrderCreate
	OrderCancel
	Financing
	FundsTransfer
)

func (c *OandaClient) SubscribeTransactions(accountID string, transType []TransactionType, handler TransactionHandler) error {

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exist := c.transactionSubscriptions[accountID]; !exist {

		transactionLogic := newTransactionTypeLogic()
		transactionLogic.hydrate(transType)
		c.transactionSubscriptions[accountID] = transactionLogic
		err := c.subscribeTransactions(accountID, handler)

		if err != nil {
			return err
		}

	} else {

		tl := c.transactionSubscriptions[accountID]

		for _, tr := range transType {
			switch tr {
			case OrderCreate:
				tl.orderCreate.Store(true)
			case OrderFill:
				tl.orderFill.Store(true)
			case OrderCancel:
				tl.orderCreate.Store(true)
			case Financing:
				tl.financing.Store(true)
			case FundsTransfer:
				tl.fundsTransfer.Store(true)
			}
		}
	}

	return nil
}

func (c *OandaClient) subscribeTransactions(accountID string, handler TransactionHandler) error {

	endpoint := "/accounts/" + accountID + "/transactions/stream"
	reader, err := c.dial(endpoint)

	if err != nil {
		return err
	}

	go func(reader *bufio.Reader, endpoint string) {

	subLoop:
		for {

			line, err := reader.ReadBytes('\n')

			if err != nil {

				logrus.Warn(err)

				if reader, err = c.reconnect(endpoint); err != nil { // Did not recover subscription, break outer loop
					break subLoop
				}

				continue
			}

			data := &Transaction{}
			err = json.Unmarshal(line, data)

			if err != nil {
				logrus.Warn(err)
				continue
			}

			if c.transactionSubscriptions[accountID].checkIgnore(data) {
				continue
			}

			handler(data)
		}
	}(reader, endpoint)

	return nil
}

func (c *OandaClient) reconnect(endpoint string) (reader *bufio.Reader, err error) {

	for i := 0; i < 3; i++ { // Try reconnection 3 times with exponential backoff

		logrus.Debug("Trying to recover subscription...")

		time.Sleep(time.Duration(math.Pow(2, float64(i))) * 100 * time.Millisecond)

		reader, err = c.dial(endpoint)

		if err == nil {
			logrus.Debug("Subscription recovered")
			return
		}
	}

	return
}
