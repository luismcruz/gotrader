package gotrader

import (
	"time"

	"go.uber.org/atomic"
)

// Trade represents a transaction in a broker (execution of an order).
// Not all brokers have the possibility to operate over single trades, making impossible to use
// this engine in the current state.
type Trade struct {
	id                        string
	instrumentName            string
	side                      Side
	units                     int32
	openTime                  time.Time
	unrealizedNetProfit       float64
	unrealizedEffectiveProfit float64
	marginUsed                float64
	leverage                  *atomic.Float64
	chargedFees               *atomic.Float64
	openPrice                 float64
	currentPrice              *atomic.Float64
	sideSign                  float64
	ccyConversion             *instrumentConversion
}

/**************************
*
*	Internal Methods
*
***************************/

func newTrade(
	inst *Instrument,
	tradeID string,
	tradeSide Side,
	tradeUnits int32,
	openTime time.Time,
	openPrice float64,
) *Trade {

	tr := &Trade{
		id:             tradeID,
		instrumentName: inst.name,
		side:           tradeSide,
		units:          tradeUnits,
		openTime:       openTime,
		openPrice:      openPrice,
		sideSign:       sideSign(tradeSide),
		ccyConversion:  inst.ccyConversion,
		leverage:       inst.leverage,
		chargedFees:    atomic.NewFloat64(0),
	}

	return tr

}

func (t *Trade) calculateUnrealized() {
	t.unrealizedNetProfit = (t.currentPrice.Load() - t.openPrice) * t.sideSign * float64(t.units) * t.ccyConversion.QuoteConversionRate.Load()
	t.unrealizedEffectiveProfit = t.unrealizedNetProfit + t.chargedFees.Load()
}

func (t *Trade) calculateMarginUsed() {
	t.marginUsed = float64(t.units) / t.leverage.Load() * t.ccyConversion.BaseConversionRate.Load()
}

func (t *Trade) updateChargedFee(fee float64) {
	t.chargedFees.Add(fee)
	t.unrealizedEffectiveProfit += fee
}

func sideSign(side Side) float64 {
	if side == Short {
		return -1.0
	}

	return 1.0
}

/**************************
*
*	Accessible Methods
*
***************************/

// ID returns the ID of the trade.
func (t *Trade) ID() string {
	return t.id
}

// InstrumentName return the instrument name.
func (t *Trade) InstrumentName() string {
	return t.instrumentName
}

// Side returns the side, it be either Long or Short
func (t *Trade) Side() Side {
	return t.side
}

// Units returns the total units that this trade is exposed.
func (t *Trade) Units() int32 {
	return t.units
}

// OpenTime returns the open time of trade.
func (t *Trade) OpenTime() time.Time {
	return t.openTime
}

// UnrealizedNetProfit returns the unrealized profit.
func (t *Trade) UnrealizedNetProfit() float64 {
	return t.unrealizedNetProfit
}

// UnrealizedEffectiveProfit returns the unrealized profit plus the charged fees.
func (t *Trade) UnrealizedEffectiveProfit() float64 {
	return t.unrealizedEffectiveProfit
}

// MarginUsed return the margin that the trade is using
func (t *Trade) MarginUsed() float64 {
	return t.marginUsed
}

// ChargedFees returns the total charged fees, like rollovers.
func (t *Trade) ChargedFees() float64 {
	return t.chargedFees.Load()
}

// OpenPrice returns the openning price of the trade.
func (t *Trade) OpenPrice() float64 {
	return t.openPrice
}

// CurrentPrice returns the current price of the trade/instrument.
func (t *Trade) CurrentPrice() float64 {
	return t.currentPrice.Load()
}
