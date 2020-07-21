package gotrader

import (
	"math"
	"time"

	"github.com/cornelk/hashmap"
	"go.uber.org/atomic"
)

// Hedge represents the type of hedging defined by the broker.
type Hedge int

const (
	FullHedge Hedge = iota
	NoHedge
	HalfHedge
)

type Instrument struct {
	name                      string
	baseCurrency              string
	quoteCurrency             string
	longPosition              *Position
	shortPosition             *Position
	tradesNumber              *atomic.Int32
	trades                    *hashmap.HashMap
	tradesTimeOrder           *sortedTrades
	unrealizedNetProfit       float64
	unrealizedEffectiveProfit float64
	marginUsed                float64
	leverage                  *atomic.Float64
	chargedFees               float64
	ask                       *atomic.Float64
	bid                       *atomic.Float64
	pipLocation               int
	ccyConversion             *instrumentConversion
	hedgeType                 Hedge
	logger                    Logger
}

/**************************
*
*	Internal Methods
*
***************************/

func newInstrument(
	name string,
	baseCurrency string,
	quoteCurrency string,
	leverage float64,
	pipLocation int,
	logger Logger,
) *Instrument {

	return &Instrument{
		name:            name,
		baseCurrency:    baseCurrency,
		quoteCurrency:   quoteCurrency,
		leverage:        atomic.NewFloat64(leverage),
		pipLocation:     pipLocation,
		longPosition:    newPosition(Long),
		shortPosition:   newPosition(Short),
		tradesNumber:    atomic.NewInt32(0),
		trades:          &hashmap.HashMap{},
		tradesTimeOrder: newSortedTrades(),
		ask:             atomic.NewFloat64(0.0),
		bid:             atomic.NewFloat64(0.0),
	}
}

func (i *Instrument) openTrade(
	id string,
	side Side,
	openTime time.Time,
	units int32,
	openPrice float64,
) *Trade {

	i.tradesNumber.Inc()

	trade := newTrade(i, id, side, units, openTime, openPrice)
	i.trades.Set(id, trade)
	i.tradesTimeOrder.Append(id)

	if side == Short {
		trade.currentPrice = i.ask
		i.shortPosition.openTrade(trade)
	} else {
		trade.currentPrice = i.bid
		i.longPosition.openTrade(trade)
	}

	return trade
}

func (i *Instrument) closeTrade(id string) {

	i.tradesNumber.Dec()

	i.tradesTimeOrder.Delete(id)
	tr, exist := i.trades.GetStringKey(id)
	if !exist {
		i.logger.Warn(i.name + ": trying to close unexisting trade")
		return
	}

	i.trades.Del(id)

	trade := tr.(*Trade)

	if trade.side == Long {
		i.longPosition.closeTrade(trade)
	} else {
		i.shortPosition.closeTrade(trade)
	}

}

func (i *Instrument) calculateUnrealized() {

	i.shortPosition.calculateUnrealized()
	i.longPosition.calculateUnrealized()

	i.unrealizedNetProfit = i.longPosition.unrealizedNetProfit + i.shortPosition.unrealizedNetProfit
	i.unrealizedEffectiveProfit = i.longPosition.unrealizedEffectiveProfit + i.shortPosition.unrealizedEffectiveProfit
	i.chargedFees = i.longPosition.chargedFees + i.shortPosition.chargedFees

}

func (i *Instrument) calculateMarginUsed() {

	i.shortPosition.calculateMarginUsed()
	i.longPosition.calculateMarginUsed()

	switch i.hedgeType {
	case NoHedge:
		i.marginUsed = i.shortPosition.marginUsed + i.longPosition.marginUsed
	case FullHedge:
		i.marginUsed = math.Abs(i.shortPosition.marginUsed - i.longPosition.marginUsed)
	case HalfHedge:
		if i.shortPosition.marginUsed > i.longPosition.marginUsed {
			i.marginUsed = i.shortPosition.marginUsed
		} else {
			i.marginUsed = i.longPosition.marginUsed
		}
	}
}

func (i *Instrument) updatePrice(tick *Tick) {
	i.ask.Store(tick.Ask)
	i.bid.Store(tick.Bid)
}

/**************************
*
*	Acessible Methods
*
***************************/

func (i *Instrument) Name() string {
	return i.name
}

func (i *Instrument) BaseCurrency() string {
	return i.baseCurrency
}

func (i *Instrument) QuoteCurrency() string {
	return i.quoteCurrency
}

func (i *Instrument) LongPosition() *Position {
	return i.longPosition
}

func (i *Instrument) ShortPosition() *Position {
	return i.shortPosition
}

func (i *Instrument) TradeByOrder(index int) *Trade {
	return i.Trade(i.tradesTimeOrder.Get(index))
}

func (i *Instrument) TradesByAscendingOrder(tradesNumber int) <-chan *Trade {

	ch := make(chan *Trade)
	go func() {
		for id := range i.tradesTimeOrder.AscendIter(tradesNumber) {
			tr, exist := i.trades.GetStringKey(id)
			if exist {
				ch <- tr.(*Trade)
			}
		}
		close(ch)
	}()

	return ch
}

func (i *Instrument) TradesByDescendingOrder(tradesNumber int) <-chan *Trade {
	ch := make(chan *Trade)
	go func() {
		for id := range i.tradesTimeOrder.DescendIter(tradesNumber) {
			tr, exist := i.trades.GetStringKey(id)
			if exist {
				ch <- tr.(*Trade)
			}
		}
		close(ch)
	}()

	return ch
}

func (i *Instrument) Trade(id string) *Trade {

	trade, exist := i.trades.GetStringKey(id)
	if exist {
		return trade.(*Trade)
	}

	return nil
}

func (i *Instrument) Trades() <-chan *Trade {

	ch := make(chan *Trade)
	go func() {
		for kv := range i.trades.Iter() {
			ch <- kv.Value.(*Trade)
		}
		close(ch)
	}()

	return ch
}

func (i *Instrument) TradesNumber() int32 {
	return i.tradesNumber.Load()
}

func (i *Instrument) UnrealizedNetProfit() float64 {
	return i.unrealizedNetProfit
}

func (i *Instrument) UnrealizedEffectiveProfit() float64 { // = UnrealizedNetProfit + ChargedFees
	return i.unrealizedEffectiveProfit
}

func (i *Instrument) MarginUsed() float64 {
	return i.marginUsed
}

func (i *Instrument) ChargedFees() float64 {
	return i.chargedFees
}

func (i *Instrument) Ask() float64 {
	return i.ask.Load()
}

func (i *Instrument) Bid() float64 {
	return i.bid.Load()
}

func (i *Instrument) Spread() float64 {
	return i.Bid() - i.Ask()
}

func (i *Instrument) Leverage() float64 {
	return i.leverage.Load()
}

func (i *Instrument) PipLocation() int {
	return i.pipLocation
}
