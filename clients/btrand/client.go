package btrand

import (
	"math/rand"
	"sort"
	"time"

	"github.com/luismcruz/gotrader"
)

type btRandClient struct {
	instruments         []gotrader.InstrumentDetails
	instrumentsPriceGen map[string]*priceGenerator
	startTime           time.Time
	endTime             time.Time
	currentTime         time.Time
}

func NewBTRandClient(instruments []gotrader.InstrumentDetails,
	startTime, endTime time.Time) gotrader.BrokerClient {

	rand.Seed(time.Now().UnixNano())

	client := &btRandClient{
		instruments:         instruments,
		instrumentsPriceGen: make(map[string]*priceGenerator),
		startTime:           startTime,
		endTime:             endTime,
		currentTime:         startTime,
	}

	return client
}

func (c *btRandClient) GetAvailableInstruments(accountID string) []gotrader.InstrumentDetails {
	return c.instruments
}

func (c *btRandClient) SubscribePrices(accountID string, instruments []gotrader.InstrumentDetails, callback gotrader.TickHandler) {

	go func() {

		for _, inst := range instruments {
			startPrice := rand.Float64()*0.6 + 0.9
			c.instrumentsPriceGen[inst.Name] = newCorePriceGenerator(inst.Name, c.startTime, startPrice, 100) //rand.Int63()
		}

		for c.currentTime.Before(c.endTime) {

			ticks := make([]*gotrader.Tick, 0, len(c.instrumentsPriceGen))

			for _, instGen := range c.instrumentsPriceGen {
				ticks = append(ticks, instGen.next())
			}

			// sort ticks
			sort.Slice(ticks, func(i, j int) bool { return ticks[i].Time.Before(ticks[j].Time) })

			// update time
			c.currentTime = ticks[len(c.instrumentsPriceGen)-1].Time

			for _, tick := range ticks {
				callback(tick)
			}

		}

		callback(nil)

	}()

}

/*******************************************************************************************************************
*
*
*									  Unused Methods (Needed for Live Only)
*
*
*******************************************************************************************************************/

func (c *btRandClient) OpenMarketOrder(accountID, instrument string, units int32, side string) {

}

func (c *btRandClient) CloseTrade(accountID, id string) {

}

func (c *btRandClient) GetOpenTrades(accountID string) []gotrader.TradeDetails {
	return nil
}

func (c *btRandClient) GetAccountStatus(accountID string) gotrader.AccountStatus {
	return gotrader.AccountStatus{}
}

func (c *btRandClient) SubscribeOrderFillNotifications(accountID string, orderFIllCallback gotrader.OrderFillHandler) {

}

func (c *btRandClient) SubscribeSwapChargeNotifications(accountID string, swapChargeCallback gotrader.SwapChargeHandler) {

}

func (c *btRandClient) SubscribeFundsTransferNotifications(accountID string, fundsTransferCallback gotrader.FundsTransferHandler) {

}
