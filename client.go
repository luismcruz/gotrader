package gotrader

import "time"

type BrokerClient interface {
	GetAccountStatus(accountID string) (AccountStatus, error)
	GetAvailableInstruments(accountID string) ([]InstrumentDetails, error)

	OpenMarketOrder(accountID, instrument string, units int32, side string) error
	CloseTrade(accountID, id string) error
	GetOpenTrades(accountID string) ([]TradeDetails, error)

	SubscribePrices(accountID string, instruments []InstrumentDetails, callback TickHandler) error
	SubscribeOrderFillNotifications(accountID string, orderFIllCallback OrderFillHandler) error
	SubscribeSwapChargeNotifications(accountID string, swapChargeCallback SwapChargeHandler) error
	SubscribeFundsTransferNotifications(accountID string, fundsTransferCallback FundsTransferHandler) error
}

type TradeDetails struct {
	ID          string
	Instrument  InstrumentDetails
	Side        Side
	Units       int32
	OpenPrice   float64
	ChargedFees float64
	OpenTime    time.Time
}

type InstrumentDetails struct {
	Name          string
	BaseCurrency  string
	QuoteCurrency string
	Leverage      float64
	PipLocation   int
}

type AccountStatus struct {
	Currency              string
	Hedge                 Hedge
	Equity                float64
	Balance               float64
	UnrealizedGrossProfit float64
	MarginUsed            float64
	MarginFree            float64
	Leverage              float64
}

type TickHandler func(tick *Tick)

type Tick struct {
	Instrument string
	Bid        float64
	Ask        float64
	Time       time.Time
}

type OrderFillHandler func(order *OrderFill)

type OrderFill struct {
	Error       string
	TradeClose  bool
	OrderID     string
	TradeID     string
	Side        Side
	Instrument  InstrumentDetails
	Price       float64
	Units       int32
	Profit      float64
	ChargedFees float64
	Time        time.Time
}

type SwapChargeHandler func(charges *SwapCharge)

type SwapCharge struct {
	Charges []*TradeSwapCharge
	Time    time.Time
}

type TradeSwapCharge struct {
	ID         string
	Ammount    float64
	Instrument InstrumentDetails
}

type FundsTransferHandler func(funds *FundsTransfer)

type FundsTransfer struct {
	Ammount float64
	Time    time.Time
}
