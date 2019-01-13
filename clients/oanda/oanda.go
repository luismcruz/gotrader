package oanda

import (
	"strings"
	"sync"

	"github.com/luismcruz/gotrader"

	"github.com/luismcruz/gotrader/clients/oanda/client"
)

type oandaClientWrapper struct {
	client                  *oandacl.OandaClient
	instrumentsDetails      map[string]gotrader.InstrumentDetails
	priceSubscription       *priceSubscription
	transactionSubscription map[string]*transactionSubscription
	mutex                   *sync.Mutex
}

// NewOandaClient is the oanda client wrapper constructor
func NewOandaClient(token string, live bool) gotrader.BrokerClient {
	return &oandaClientWrapper{
		client:                  oandacl.NewClient(token, live),
		instrumentsDetails:      make(map[string]gotrader.InstrumentDetails),
		transactionSubscription: make(map[string]*transactionSubscription),
		mutex: &sync.Mutex{},
	}
}

func (c *oandaClientWrapper) GetAccountStatus(accountID string) (gotrader.AccountStatus, error) {

	accountSummary, err := c.client.GetAccountSummary(accountID)

	if err != nil {
		return gotrader.AccountStatus{}, err
	}

	resp := gotrader.AccountStatus{
		Currency:              accountSummary.Account.Currency,
		Hedge:                 gotrader.HalfHedge,
		Equity:                accountSummary.Account.NAV,
		Balance:               accountSummary.Account.Balance,
		UnrealizedGrossProfit: accountSummary.Account.UnrealizedPL,
		MarginUsed:            accountSummary.Account.MarginUsed,
		MarginFree:            accountSummary.Account.MarginAvailable,
		Leverage:              1.0 / accountSummary.Account.MarginRate,
	}

	return resp, nil
}

func (c *oandaClientWrapper) GetAvailableInstruments(accountID string) ([]gotrader.InstrumentDetails, error) {

	oandaResp, err := c.client.GetAccountInstruments(accountID)

	if err != nil {
		return nil, err
	}

	instruments := oandaResp.Instruments

	resp := make([]gotrader.InstrumentDetails, len(instruments), len(instruments))

	for i, inst := range instruments {

		ccys := strings.Split(inst.Name, "_")

		newInst := gotrader.InstrumentDetails{
			Name:          inst.Name,
			BaseCurrency:  ccys[0],
			QuoteCurrency: ccys[1],
			Leverage:      1 / inst.MarginRate,
			PipLocation:   inst.PipLocation,
		}

		if _, exist := c.instrumentsDetails[inst.Name]; !exist {
			c.instrumentsDetails[inst.Name] = newInst
		}

		resp[i] = newInst
	}

	return resp, nil

}

func (c *oandaClientWrapper) OpenMarketOrder(accountID, instrument string, units int32, side string) error {

	_, err := c.client.CreateMarketOrder(accountID, instrument, side, units)

	if err != nil {
		return err
	}

	return nil
}

func (c *oandaClientWrapper) CloseTrade(accountID, id string) error {

	_, err := c.client.CloseTrade(accountID, id)

	if err != nil {
		return err
	}

	return nil
}

func (c *oandaClientWrapper) GetOpenTrades(accountID string) ([]gotrader.TradeDetails, error) {

	tradesResp, err := c.client.GetOpenTrades(accountID)

	if err != nil {
		return nil, err
	}

	response := make([]gotrader.TradeDetails, len(tradesResp.Trades), len(tradesResp.Trades))

	for i, tr := range tradesResp.Trades {

		var side gotrader.Side

		if tr.CurrentUnits > 0 {
			side = gotrader.Long
		} else {
			side = gotrader.Short
			tr.CurrentUnits = -tr.CurrentUnits
		}

		response[i] = gotrader.TradeDetails{
			ID:          tr.ID,
			Instrument:  c.instrumentsDetails[tr.Instrument],
			Side:        side,
			Units:       tr.CurrentUnits,
			OpenPrice:   tr.Price,
			ChargedFees: tr.Financing,
			OpenTime:    tr.OpenTime,
		}

	}

	return response, nil

}

func (c *oandaClientWrapper) SubscribePrices(accountID string, instruments []gotrader.InstrumentDetails, callback gotrader.TickHandler) error {

	instrumentsStrings := make([]string, len(instruments), len(instruments))

	for i, inst := range instruments {
		instrumentsStrings[i] = inst.Name
	}

	if c.priceSubscription == nil {
		c.priceSubscription = &priceSubscription{handler: callback}
	}

	err := c.client.SubscribePrices(accountID, instrumentsStrings, c.priceSubscription.priceHandler)

	if err != nil {
		return err
	}

	return nil
}

func (c *oandaClientWrapper) SubscribeOrderFillNotifications(accountID string, orderFillCallback gotrader.OrderFillHandler) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exist := c.transactionSubscription[accountID]; !exist {
		c.transactionSubscription[accountID] = new(transactionSubscription)
		c.transactionSubscription[accountID].insturmentDetails = c.instrumentsDetails
	}

	subscription := c.transactionSubscription[accountID]
	subscription.orderFillCallback = orderFillCallback

	err := c.client.SubscribeTransactions(accountID, []oandacl.TransactionType{oandacl.OrderFill}, subscription.transactionHandler)

	if err != nil {
		return err
	}

	return nil
}

func (c *oandaClientWrapper) SubscribeSwapChargeNotifications(accountID string, swapChargeCallback gotrader.SwapChargeHandler) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exist := c.transactionSubscription[accountID]; !exist {
		c.transactionSubscription[accountID] = new(transactionSubscription)
		c.transactionSubscription[accountID].insturmentDetails = c.instrumentsDetails
	}

	subscription := c.transactionSubscription[accountID]
	subscription.swapChargeCallback = swapChargeCallback

	err := c.client.SubscribeTransactions(accountID, []oandacl.TransactionType{oandacl.Financing}, subscription.transactionHandler)

	if err != nil {
		return err
	}

	return nil
}

func (c *oandaClientWrapper) SubscribeFundsTransferNotifications(accountID string, fundsTransferCallback gotrader.FundsTransferHandler) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exist := c.transactionSubscription[accountID]; !exist {
		c.transactionSubscription[accountID] = new(transactionSubscription)
		c.transactionSubscription[accountID].insturmentDetails = c.instrumentsDetails
	}

	subscription := c.transactionSubscription[accountID]
	subscription.fundsTransferCallback = fundsTransferCallback

	err := c.client.SubscribeTransactions(accountID, []oandacl.TransactionType{oandacl.FundsTransfer}, subscription.transactionHandler)

	if err != nil {
		return err
	}

	return nil
}

type priceSubscription struct {
	handler gotrader.TickHandler
}

func (p *priceSubscription) priceHandler(price oandacl.Price) {

	tick := &gotrader.Tick{
		Instrument: price.Instrument,
		Bid:        price.Bids[0].Price,
		Ask:        price.Asks[0].Price,
		Time:       price.Time,
	}

	p.handler(tick)
}

type transactionSubscription struct {
	insturmentDetails     map[string]gotrader.InstrumentDetails
	orderFillCallback     gotrader.OrderFillHandler
	swapChargeCallback    gotrader.SwapChargeHandler
	fundsTransferCallback gotrader.FundsTransferHandler
}

func (t *transactionSubscription) transactionHandler(transaction *oandacl.Transaction) {

	if transaction.Type == "ORDER_FILL" && t.orderFillCallback != nil {

		var (
			orderFill *gotrader.OrderFill
			side      = gotrader.Long
		)

		if transaction.RejectReason != nil {

			orderFill = &gotrader.OrderFill{
				Error: *transaction.RejectReason,
			}

			t.orderFillCallback(orderFill)

		} else if transaction.TradesClosed != nil {

			for _, trade := range transaction.TradesClosed {

				if trade.Units > 0 { // Closing short trade
					side = gotrader.Short
				} else {
					trade.Units = -trade.Units
				}

				orderFill = &gotrader.OrderFill{
					TradeClose:  true,
					OrderID:     transaction.OrderID,
					TradeID:     trade.TradeID,
					Side:        side,
					Instrument:  t.insturmentDetails[transaction.Instrument],
					Price:       trade.Price,
					Units:       trade.Units,
					Profit:      trade.RealizedPL,
					ChargedFees: trade.Financing,
					Time:        transaction.Time,
				}

				t.orderFillCallback(orderFill)
			}
		} else if transaction.TradeOpened != nil {

			if transaction.TradeOpened.Units < 0 {
				side = gotrader.Short
				transaction.TradeOpened.Units = -transaction.TradeOpened.Units
			}

			orderFill = &gotrader.OrderFill{
				TradeClose: false,
				OrderID:    transaction.OrderID,
				TradeID:    transaction.TradeOpened.TradeID,
				Side:       side,
				Instrument: t.insturmentDetails[transaction.Instrument],
				Price:      transaction.TradeOpened.Price,
				Units:      transaction.TradeOpened.Units,
				Time:       transaction.Time,
			}

			t.orderFillCallback(orderFill)
		}

	} else if transaction.Type == "TRANSFER_FUNDS" && t.fundsTransferCallback != nil {

		fundsTransfer := &gotrader.FundsTransfer{
			Ammount: transaction.Amount,
			Time:    transaction.Time,
		}

		t.fundsTransferCallback(fundsTransfer)

	} else if transaction.Type == "DAILY_FINANCING" && t.swapChargeCallback != nil {

		var (
			length int
			idx    int
		)

		for _, pf := range transaction.PositionFinancings {
			length += len(pf.TradesFinancing)
		}

		tradesSwapCharges := make([]*gotrader.TradeSwapCharge, length, length)

		for _, pf := range transaction.PositionFinancings {
			for _, tf := range pf.TradesFinancing {

				tradeCharge := &gotrader.TradeSwapCharge{
					ID:         tf.TradeID,
					Ammount:    tf.Financing,
					Instrument: t.insturmentDetails[pf.Instrument],
				}

				tradesSwapCharges[idx] = tradeCharge
				idx++
			}
		}

		charge := &gotrader.SwapCharge{
			Charges: tradesSwapCharges,
			Time:    transaction.Time,
		}

		t.swapChargeCallback(charge)
	}
}
