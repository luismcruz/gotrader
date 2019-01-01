package oandacl

import (
	"time"
)

type Trades struct {
	Trades []Trade `json:"trades"`
}

type Trade struct {
	CurrentUnits int32     `json:"currentUnits,string"`
	Financing    float64   `json:"financing,string"`
	ID           string    `json:"id"`
	InitialUnits int32     `json:"initialUnits,string"`
	Instrument   string    `json:"instrument"`
	OpenTime     time.Time `json:"openTime"`
	Price        float64   `json:"price,string"`
	RealizedPL   float64   `json:"realizedPL,string"`
	State        string    `json:"state"`
	UnrealizedPL float64   `json:"unrealizedPL,string"`
}

type CloseTradeResponse struct {
	OrderCreateTransaction *OrderCreateTransaction `json:"orderCreateTransaction"`
	OrderFillTransaction   *OrderFillTransaction   `json:"orderFillTransaction"`
	OrderCancelTransaction *OrderCancelTransaction `json:"orderCancelTransaction"`
}

type TradeReduced struct {
	TradeID    string  `json:"tradeID"`
	Units      int32   `json:"units,string"`
	Price      float64 `json:"price,string"`
	RealizedPL float64 `json:"realizedPL,string"`
	Financing  float64 `json:"financing,string"`
}

type OrderCancelTransaction struct {
	Type      string    `json:"type"`
	OrderID   string    `json:"orderID"`
	Reason    string    `json:"reason"`
	ID        string    `json:"id"`
	UserID    int       `json:"userID"`
	AccountID string    `json:"accountID"`
	BatchID   string    `json:"batchID"`
	RequestID string    `json:"requestID"`
	Time      time.Time `json:"time"`
}

func (c *OandaClient) GetOpenTrades(accountID string) Trades {
	endpoint := "/accounts/" + accountID + "/openTrades"

	response := c.get(endpoint)
	data := Trades{}
	unmarshalJSON(response, &data)

	return data
}

func (c *OandaClient) CloseTrade(accountID, tradeID string) CloseTradeResponse {

	endpoint := "/accounts/" + accountID + "/trades/" + tradeID + "/close"

	response := c.put(endpoint)
	data := CloseTradeResponse{}
	unmarshalJSON(response, &data)

	return data

}
