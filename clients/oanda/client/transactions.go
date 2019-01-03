package oandacl

import (
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
	Units              int32                `json:"units,string"`
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

func (c *OandaClient) SubscribeTransactions(accountID string, transType []TransactionType, handler TransactionHandler) {

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exist := c.transactionSubscriptions[accountID]; !exist {

		transactionLogic := newTransactionTypeLogic()
		transactionLogic.hydrate(transType)
		c.transactionSubscriptions[accountID] = transactionLogic
		c.subscribeTransactions(accountID, handler)

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

}

func (c *OandaClient) subscribeTransactions(accountID string, handler TransactionHandler) {

	go func() {

		endpoint := "/accounts/" + accountID + "/transactions/stream"
		reader := c.subscribe(endpoint)

		for {

			line, err := reader.ReadBytes('\n')

			if err != nil {
				logrus.Warn(err)
				continue
			}

			data := &Transaction{}
			unmarshalJSON(line, data)

			if c.transactionSubscriptions[accountID].checkIgnore(data) {
				continue
			}

			//logrus.Info(string(line))

			handler(data)

		}
	}()

}
