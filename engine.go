package gotrader

import (
	"math"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"syscall"
	"time"

	"github.com/uber-go/atomic"

	"github.com/sirupsen/logrus"
)

// Engine is the interface used for interaction from strategy.
// Is used to check the state of the account, open or close trades and to stop the session.
type Engine interface {
	Account() *Account
	Buy(instrument string, units int32)
	Sell(instrument string, units int32)
	CloseTrade(instrument string, id string)
	StopSession() // Gracefully stops trading session from strategy
}

/***********************************************************************************************
*
*											Live Engine
*
************************************************************************************************/

type liveEngine struct {
	account                  *Account
	client                   BrokerClient
	parameters               *sessionParameters
	strategy                 Strategy
	currencyConversionEngine *currencyConversionEngine
	availableInstrumentsMap  map[string]InstrumentDetails
	ticks                    chan *Tick
	orders                   chan *OrderFill
	fundsTransfers           chan *FundsTransfer
	swapCharges              chan *SwapCharge
	ready                    bool
	endOfSession             chan bool
}

func newLiveEngine() *liveEngine {
	return &liveEngine{
		ticks:                   make(chan *Tick, 300),
		orders:                  make(chan *OrderFill, 100),
		fundsTransfers:          make(chan *FundsTransfer, 100),
		swapCharges:             make(chan *SwapCharge, 100),
		availableInstrumentsMap: make(map[string]InstrumentDetails),
		endOfSession:            make(chan bool, 1),
	}
}

func (e *liveEngine) start() error {

	e.account = newAccount(e.parameters.account)

	// Account Status Retrieval
	accountStatus, err := e.client.GetAccountStatus(e.parameters.account)

	if err != nil {
		return err
	}

	e.account.balance.Store(accountStatus.Balance)
	e.account.homeCurrency = accountStatus.Currency
	e.account.leverage = accountStatus.Leverage

	// Initialize Trading Instruments
	availableInstruments, err := e.client.GetAvailableInstruments(e.account.id)

	if err != nil {
		return err
	}

	conversionInstruments := make(map[string]*instrumentConversion)

	for _, inst := range availableInstruments {

		e.availableInstrumentsMap[inst.Name] = inst

		for _, trdInst := range e.parameters.instruments {

			if inst.Name == trdInst {

				e.account.instruments[inst.Name] = newInstrument(inst.Name,
					inst.BaseCurrency,
					inst.QuoteCurrency,
					math.Min(inst.Leverage, e.account.leverage),
				)
				e.account.instruments[inst.Name].hedgeType = accountStatus.Hedge

				conversionInstruments[inst.Name] = newInstrumentConversion(inst.Name,
					inst.BaseCurrency,
					inst.QuoteCurrency,
				)

				break
			}
		}
	}

	// Initialize Currency Conversion Engine
	e.currencyConversionEngine = newCurrencyConversionEngine(conversionInstruments,
		e.availableInstrumentsMap, e.account.homeCurrency)
	e.currencyConversionEngine.start()

	e.currencyConversionEngine.setPricePointers(e.account.instruments)

	// Hydrate current positions state from Broker sorted by open time
	trades, err := e.client.GetOpenTrades(e.account.id)

	if err != nil {
		return err
	}

	sort.Slice(trades, func(i, j int) bool { return trades[i].OpenTime.Before(trades[j].OpenTime) })

	for _, t := range trades {

		inst, exist := e.account.instruments[t.Instrument.Name]
		if exist {
			inst.openTrade(t.ID, t.Side, t.OpenTime, t.Units, t.OpenPrice)
			inst.Trade(t.ID).chargedFees = t.ChargedFees
		}
	}

	// Subscribe prices
	err = e.client.SubscribePrices(e.account.id, e.currencyConversionEngine.conversionInstrumentsDetails, e.onTick)

	if err != nil {
		return err
	}

	// Subscribe notifications
	err = e.client.SubscribeOrderFillNotifications(e.account.id, e.onOrderFill)

	if err != nil {
		return err
	}

	err = e.client.SubscribeSwapChargeNotifications(e.account.id, e.onSwapCharge)

	if err != nil {
		return err
	}

	err = e.client.SubscribeFundsTransferNotifications(e.account.id, e.onFundsTransfer)

	if err != nil {
		return err
	}

	// Initialize consumers (buffered channels are used to prevent race conditions)
	e.startOrderFillConsumer()
	e.startSwapChargesConsumer()
	e.startFundsTransferConsumer()

	// Initialize strategy
	e.strategy.SetEngine(e)
	e.strategy.Initialize()

	// Run strategy OnStop method when the app receives the signal to shutdown
	e.shutdownHook()

	// Run strategy
	e.run()

	// Stop strategy
	e.strategy.OnStop()

	return nil
}

func (e *liveEngine) shutdownHook() {
	var singalChan = make(chan os.Signal)
	signal.Notify(singalChan, syscall.SIGTERM)
	signal.Notify(singalChan, syscall.SIGINT)

	go func() {
		<-singalChan
		e.strategy.OnStop()
		os.Exit(0)
	}()
}

func (e *liveEngine) onTick(tick *Tick) { // Ticks callback

	select { // non blocking buffered channel
	case e.ticks <- tick:
	default: // Replaces older ticks by newer ones (extreme case)
		<-e.ticks
		e.ticks <- tick
	}

}

func (e *liveEngine) onOrderFill(orderFill *OrderFill) { // Orders callback
	e.orders <- orderFill
}

func (e *liveEngine) onSwapCharge(swapCharge *SwapCharge) { // Swap/Rollover charges callback
	e.swapCharges <- swapCharge
}

func (e *liveEngine) onFundsTransfer(funds *FundsTransfer) { // Funds transfer callback
	e.fundsTransfers <- funds
}

func (e *liveEngine) startOrderFillConsumer() {

	go func() {
		for orderFill := range e.orders {

			if orderFill.Error == "" {
				if !orderFill.TradeClose {
					e.account.instruments[orderFill.Instrument.Name].openTrade(orderFill.TradeID,
						orderFill.Side, orderFill.Time, orderFill.Units, orderFill.Price)
				} else {
					e.account.instruments[orderFill.Instrument.Name].closeTrade(orderFill.TradeID)
					e.account.balance.Add(orderFill.Profit)
				}
			}

			e.strategy.OnOrderFill(orderFill)
		}
	}()
}

func (e *liveEngine) startSwapChargesConsumer() {

	go func() {
		for swapCharge := range e.swapCharges {
			for _, charge := range swapCharge.Charges {

				tr, exist := e.account.instruments[charge.Instrument.Name].trades.Get(charge.ID)

				if !exist {
					logrus.Warn(charge, "charging swap on unexisting trade")
					continue
				}

				trade := tr.(*Trade)
				trade.chargedFees += charge.Ammount

				e.account.balance.Add(charge.Ammount)
			}
		}
	}()

}

func (e *liveEngine) startFundsTransferConsumer() {

	go func() {
		for funds := range e.fundsTransfers {
			e.account.balance.Add(funds.Ammount)
		}
	}()

}

func (e *liveEngine) run() {

	for { // Application blocks until end of session

		select {
		case <-e.endOfSession:
			return
		case tick := <-e.ticks:

			if _, exist := e.account.instruments[tick.Instrument]; exist {

				e.account.instruments[tick.Instrument].updatePrice(tick)
				e.currencyConversionEngine.updateRate(tick.Instrument)
				e.account.time = tick.Time

				if e.ready {

					e.account.calculateUnrealized()
					e.account.calculateMarginUsed()
					e.account.calculateFreeMargin()

					e.strategy.OnTick(tick)
				} else {
					e.checkState()
				}

			} else { // This is the auxiliar instrument update (price state is kept only on the ccyconv engine)

				if _, exist := e.currencyConversionEngine.conversionInstruments[tick.Instrument]; exist {

					inst := e.currencyConversionEngine.conversionInstruments[tick.Instrument]
					inst.Bid.Store(tick.Bid)
					inst.Ask.Store(tick.Ask)
					e.currencyConversionEngine.updateRate(tick.Instrument)
				} else {
					logrus.Warn("received a tick from an instrument that was not subscribed and it has been ignored")
				}
			}
		}
	}
}

// Check if all instruments have already a price defined
func (e *liveEngine) checkState() {
	for _, inst := range e.currencyConversionEngine.conversionInstruments {
		if inst.Ask.Load() == 0.0 {
			return
		}
	}

	e.ready = true
}

func (e *liveEngine) calcMarginUsed(instrument string, units int32) float64 {

	leverage := e.account.instruments[instrument].leverage
	conversionRate := e.account.instruments[instrument].ccyConversion.BaseConversionRate.Load()
	marginUsed := float64(units) / leverage.Load() / conversionRate

	return marginUsed
}

/**************************
*
*	Accessible Methods
*
***************************/

func (e *liveEngine) Account() *Account {
	return e.account
}

func (e *liveEngine) Buy(instrument string, units int32) {

	go func() {

		if e.calcMarginUsed(instrument, units) > e.account.marginFree { // Only send request if there is enough margin
			e.orders <- &OrderFill{
				Error:      "NOT_ENOUGH_MARGIN",
				Instrument: e.availableInstrumentsMap[instrument],
				Side:       Long,
				Units:      units,
				Time:       time.Now(),
			}
			return
		}

		err := e.client.OpenMarketOrder(e.account.id, instrument, units, Long.String())

		if err != nil {
			e.orders <- &OrderFill{
				Error:      err.Error(),
				Instrument: e.availableInstrumentsMap[instrument],
				Side:       Short,
				Units:      units,
				Time:       time.Now(),
			}
		}

	}()

}

func (e *liveEngine) Sell(instrument string, units int32) {

	go func() {

		if e.calcMarginUsed(instrument, units) > e.account.marginFree { // Only send request if there is enough margin
			e.orders <- &OrderFill{
				Error:      "NOT_ENOUGH_MARGIN",
				Instrument: e.availableInstrumentsMap[instrument],
				Side:       Short,
				Units:      units,
				Time:       time.Now(),
			}
			return
		}

		err := e.client.OpenMarketOrder(e.account.id, instrument, units, Short.String())

		if err != nil {
			e.orders <- &OrderFill{
				Error:      err.Error(),
				Instrument: e.availableInstrumentsMap[instrument],
				Side:       Short,
				Units:      units,
				Time:       time.Now(),
			}
		}

	}()

}

func (e *liveEngine) CloseTrade(instrument, id string) {

	go func() {

		err := e.client.CloseTrade(e.account.id, id)

		if err != nil {
			e.orders <- &OrderFill{
				Error:      err.Error(),
				Instrument: e.availableInstrumentsMap[instrument],
				TradeID:    id,
				Time:       time.Now(),
			}
		}

	}()

}

func (e *liveEngine) StopSession() {
	e.endOfSession <- true
}

/***********************************************************************************************
*
*											Backtest Engine
*
*
*
************************************************************************************************/

type btEngine struct {
	account                  *Account
	client                   BrokerClient
	parameters               *sessionParameters
	strategy                 Strategy
	currencyConversionEngine *currencyConversionEngine
	ticks                    chan *Tick
	tradesCounter            *atomic.Int32
	instrumentsDetails       map[string]InstrumentDetails
	ready                    bool
	endOfSession             chan bool
}

func newBtEngine() *btEngine {
	return &btEngine{
		ticks:              make(chan *Tick, 300),
		tradesCounter:      atomic.NewInt32(0),
		instrumentsDetails: make(map[string]InstrumentDetails),
		endOfSession:       make(chan bool, 1),
	}
}

func (e *btEngine) start() error {

	e.account = newAccount(e.parameters.account)

	// Account Status Retrieval
	e.account.balance.Store(e.parameters.testParameters.initialBalance)
	e.account.homeCurrency = e.parameters.testParameters.homeCurrency
	e.account.leverage = e.parameters.testParameters.leverage

	// Initialize Trading Instruments
	availableInstruments, err := e.client.GetAvailableInstruments(e.account.id)

	if err != nil {
		return err
	}

	availableInstrumentsMap := make(map[string]InstrumentDetails)

	conversionInstruments := make(map[string]*instrumentConversion)

	for _, inst := range availableInstruments {

		availableInstrumentsMap[inst.Name] = inst

		for _, trdInst := range e.parameters.instruments {

			if inst.Name == trdInst {

				e.instrumentsDetails[inst.Name] = inst

				e.account.instruments[inst.Name] = newInstrument(inst.Name,
					inst.BaseCurrency,
					inst.QuoteCurrency,
					math.Min(inst.Leverage, e.account.leverage),
				)
				e.account.instruments[inst.Name].hedgeType = e.parameters.testParameters.hedge

				conversionInstruments[inst.Name] = newInstrumentConversion(inst.Name,
					inst.BaseCurrency,
					inst.QuoteCurrency,
				)

				break
			}
		}
	}

	// Initialize Currency Conversion Engine
	e.currencyConversionEngine = newCurrencyConversionEngine(conversionInstruments,
		availableInstrumentsMap, e.account.homeCurrency)
	e.currencyConversionEngine.start()

	e.currencyConversionEngine.setPricePointers(e.account.instruments)

	// Subscribe prices
	err = e.client.SubscribePrices(e.account.id, e.currencyConversionEngine.conversionInstrumentsDetails, e.onTick)

	if err != nil {
		return err
	}

	// Initialize strategy
	e.strategy.SetEngine(e)
	e.strategy.Initialize()

	// Run strategy
	e.run()

	// Stop strategy
	e.strategy.OnStop()

	return nil
}

func (e *btEngine) onTick(tick *Tick) { // Ticks callback
	e.ticks <- tick
}

func (e *btEngine) onOrderOpen(instrument string, units int32, side Side) {

	var (
		price float64
		order *OrderFill
	)

	if side == Long {
		price = e.account.instruments[instrument].Ask()
	} else {
		price = e.account.instruments[instrument].Bid()
	}

	leverage := e.account.instruments[instrument].leverage
	conversionRate := e.account.instruments[instrument].ccyConversion.BaseConversionRate.Load()
	marginUsed := float64(units) / leverage.Load() / conversionRate

	tradeID := strconv.FormatInt(int64(e.tradesCounter.Inc()), 10)
	time := e.account.time

	if marginUsed < e.account.marginFree {

		e.account.instruments[instrument].openTrade(
			tradeID,
			side,
			time,
			units,
			price,
		)

		e.account.calculateMarginUsed()
		e.account.calculateFreeMargin()

		order = &OrderFill{
			TradeClose:  false,
			OrderID:     tradeID,
			TradeID:     tradeID,
			Side:        side,
			Instrument:  e.instrumentsDetails[instrument],
			Price:       price,
			Units:       units,
			Profit:      0.0,
			ChargedFees: 0.0,
			Time:        time,
		}

	} else {
		order = &OrderFill{
			Error: "NOT_ENOUGH_MARGIN",
		}
	}

	e.strategy.OnOrderFill(order)
}

func (e *btEngine) onCloseTrade(tradeID, instrument string) {

	var (
		order *OrderFill
	)

	tr := e.account.instruments[instrument].Trade(tradeID)

	if tr != nil {

		e.account.balance.Add(tr.unrealizedEffectiveProfit)
		e.account.instruments[instrument].closeTrade(tradeID)
		e.account.calculateUnrealized()
		e.account.calculateMarginUsed()
		e.account.calculateFreeMargin()

		order = &OrderFill{
			Error:       "",
			TradeClose:  true,
			OrderID:     tradeID,
			TradeID:     tradeID,
			Side:        tr.side,
			Instrument:  e.instrumentsDetails[instrument],
			Price:       tr.CurrentPrice(),
			Units:       tr.units,
			Profit:      tr.unrealizedNetProfit,
			ChargedFees: 0.0,
			Time:        e.account.time,
		}

	} else {

		order = &OrderFill{
			Error:      "TRADE_DOES_NOT_EXIST",
			TradeClose: true,
			Time:       e.account.time,
		}
	}

	e.strategy.OnOrderFill(order)

}

func (e *btEngine) run() {

	for { // Application blocks until ticks channel is closed

		select {
		case <-e.endOfSession:
			return
		case tick := <-e.ticks:

			if tick == nil {
				return
			}

			if _, exist := e.account.instruments[tick.Instrument]; exist {

				e.account.instruments[tick.Instrument].updatePrice(tick)
				e.currencyConversionEngine.updateRate(tick.Instrument)
				e.account.time = tick.Time

				if e.ready {

					e.account.calculateUnrealized()
					e.account.calculateMarginUsed()
					e.account.calculateFreeMargin()

					e.strategy.OnTick(tick)
				} else {
					e.checkState()
				}

			} else { // This is the auxiliar instrument update (price state is kept only on the ccyconv engine)

				if _, exist := e.currencyConversionEngine.conversionInstruments[tick.Instrument]; exist {

					inst := e.currencyConversionEngine.conversionInstruments[tick.Instrument]
					inst.Bid.Store(tick.Bid)
					inst.Ask.Store(tick.Ask)
					e.currencyConversionEngine.updateRate(tick.Instrument)
				} else {
					logrus.Warn("received a tick from an instrument that was not subscribed and it has been ignored")
				}
			}
		}
	}
}

// Check if all instruments have already a price defined
func (e *btEngine) checkState() {
	for _, inst := range e.currencyConversionEngine.conversionInstruments {
		if inst.Ask == nil {
			return
		}
	}

	e.ready = true
}

/**************************
*
*	Accessible Methods
*
***************************/

func (e *btEngine) Account() *Account {
	return e.account
}

func (e *btEngine) Buy(instrument string, units int32) {

	e.onOrderOpen(instrument, units, Long)

}

func (e *btEngine) Sell(instrument string, units int32) {

	e.onOrderOpen(instrument, units, Short)

}

func (e *btEngine) CloseTrade(instrument, id string) {

	e.onCloseTrade(id, instrument)

}

func (e *btEngine) StopSession() {
	e.endOfSession <- true
}
