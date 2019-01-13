package oandacl

import (
	"encoding/json"
	"time"
)

type ClientExtensions struct {
	Comment *string `json:"comment,omitempty"`
	ID      *string `json:"id,omitempty"`
	Tag     *string `json:"tag,omitempty"`
}

type Order struct {
	Units            int32             `json:"units,string"`
	Instrument       string            `json:"instrument"`
	TimeInForce      string            `json:"timeInForce"`
	Type             string            `json:"type"`
	PositionFill     string            `json:"positionFill,omitempty"`
	Price            float64           `json:"price,omitempty"`
	ClientExtensions *ClientExtensions `json:"tradeClientExtensions,omitempty"`
}

type OrderRequest struct {
	Order Order `json:"order"`
}
type OrderResponse struct {
	OrderCreateTransaction *OrderCreateTransaction `json:"orderCreateTransaction"`
	OrderFillTransaction   *OrderFillTransaction   `json:"orderFillTransaction"`
}

type OrderCreateTransaction struct {
	AccountID    string                 `json:"accountID"`
	ID           string                 `json:"id"`
	Instrument   string                 `json:"instrument"`
	PositionFill string                 `json:"positionFill"`
	Reason       string                 `json:"reason"`
	TradeClose   *MarketOrderTradeClose `json:"tradeClose"`
	Time         time.Time              `json:"time"`
	TimeInForce  string                 `json:"timeInForce"`
	Type         string                 `json:"type"`
	Units        int32                  `json:"units,string"`
}

type MarketOrderTradeClose struct {
	TradeID string `json:"tradeID"`
	Units   int32  `json:"units,string"`
}

type OrderFillTransaction struct {
	Type           string          `json:"type"`
	Instrument     string          `json:"instrument"`
	Units          string          `json:"units"`
	PL             string          `json:"pl"`
	Financing      string          `json:"financing"`
	Commission     string          `json:"commission"`
	AccountBalance string          `json:"accountBalance"`
	TimeInForce    string          `json:"timeInForce"`
	PositionFill   string          `json:"positionFill"`
	Reason         string          `json:"reason"`
	TradesClosed   []*TradeReduced `json:"tradesClosed"`
	TradeReduced   *TradeReduced   `json:"tradeReduced"`
	TradeOpened    *TradeOpened    `json:"tradeOpened"`
	ID             string          `json:"id"`
	UserID         int             `json:"userID"`
	AccountID      string          `json:"accountID"`
	BatchID        string          `json:"batchID"`
	RequestID      string          `json:"requestID"`
	OrderID        string          `json:"orderId"`
	ClientOrderID  string          `json:"clientOrderId"`
	Time           time.Time       `json:"time"`
}

type TradeOpened struct {
	TradeID string  `json:"tradeID"`
	Units   int32   `json:"units,string"`
	Price   float64 `json:"price,string"`
}

func (c *OandaClient) CreateMarketOrder(accountID, instrument, side string, units int32) (OrderResponse, error) {

	if side == "SHORT" {
		units = -units
	}

	order := Order{
		Units:        units,
		Instrument:   instrument,
		TimeInForce:  "FOK",
		Type:         "MARKET",
		PositionFill: "DEFAULT",
	}

	body := OrderRequest{Order: order}

	endpoint := "/accounts/" + accountID + "/orders"

	jsonBody, err := json.Marshal(body)

	if err != nil {
		return OrderResponse{}, err
	}

	response, err := c.post(endpoint, jsonBody)

	if err != nil {
		return OrderResponse{}, err
	}

	data := OrderResponse{}
	err = json.Unmarshal(response, &data)

	if err != nil {
		return OrderResponse{}, err
	}

	return data, nil

}
