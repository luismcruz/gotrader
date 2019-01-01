package gotrader

import "github.com/cornelk/hashmap"

// Side represents the type of position, which can be short or long
type Side int

const (
	//Short represents a selling position
	Short Side = iota

	// Long represents a buying position
	Long
)

func (s Side) String() string {

	names := [...]string{"SHORT", "LONG"}

	return names[s]
}

// Position represents the total exposure in a single side of an instrument.
// Is the aggregation of all the trades of that side.
type Position struct {
	side                      Side
	trades                    *hashmap.HashMap
	tradesTimeOrder           *sortedTrades
	tradesNumber              int32
	units                     int32
	unrealizedNetProfit       float64
	unrealizedEffectiveProfit float64
	marginUsed                float64
	chargedFees               float64
	averagePrice              float64
}

/**************************
*
*	Internal Methods
*
***************************/

func newPosition(side Side, leverage *float64) *Position {
	return &Position{
		side:            side,
		trades:          &hashmap.HashMap{},
		tradesTimeOrder: newSortedTrades(),
	}
}

func (p *Position) openTrade(trade *Trade) {
	p.tradesTimeOrder.Append(trade.id)
	p.trades.Set(trade.id, trade)
	p.tradesNumber++
	p.averagePrice = (p.averagePrice*float64(p.units) + trade.openPrice*float64(trade.units)) / float64(p.units+trade.units)
	p.units += trade.units
	trade.calculateMarginUsed()
	p.marginUsed += trade.marginUsed

}

func (p *Position) closeTrade(trade *Trade) {
	p.tradesTimeOrder.Delete(trade.id)
	p.trades.Del(trade.id)
	p.tradesNumber--
	p.averagePrice = (p.averagePrice*float64(p.units) - trade.openPrice*float64(trade.units)) / float64(p.units-trade.units)
	p.units -= trade.units
	trade.calculateMarginUsed()
	p.marginUsed -= trade.marginUsed

}

func (p *Position) calculateUnrealized() {

	unrealizedNet := 0.0
	unrealizedEffective := 0.0
	averagePrice := 0.0 // recalculate to prevent possible cumulative errors
	totalUnits := 0.0
	chargedFees := 0.0

	for kv := range p.trades.Iter() {

		trade := kv.Value.(*Trade)

		trade.calculateUnrealized()

		unrealizedNet += trade.unrealizedNetProfit
		unrealizedEffective += trade.unrealizedEffectiveProfit
		chargedFees += trade.chargedFees

		averagePrice = (averagePrice*totalUnits + trade.openPrice*float64(trade.units)) / (totalUnits + float64(trade.units))
		totalUnits += float64(trade.units)
	}

	p.unrealizedNetProfit = unrealizedNet
	p.unrealizedEffectiveProfit = unrealizedEffective
	p.averagePrice = averagePrice
	p.chargedFees = chargedFees

}

func (p *Position) calculateMarginUsed() {

	marginUsed := 0.0

	for kv := range p.trades.Iter() {

		trade := kv.Value.(*Trade)

		trade.calculateMarginUsed()
		marginUsed += trade.marginUsed

	}

	p.marginUsed = marginUsed
}

/**************************
*
*	Accessible Methods
*
***************************/

func (p *Position) Side() Side {
	return p.side
}

func (p *Position) TradeByOrder(index int) string {
	return p.tradesTimeOrder.Get(index)
}

func (p *Position) TradesByAscendingOrder(tradesNumber int) <-chan string {
	return p.tradesTimeOrder.AscendIter(tradesNumber)
}

func (p *Position) TradesByDescendingOrder(tradesNumber int) <-chan string {
	return p.tradesTimeOrder.DescendIter(tradesNumber)
}

func (p *Position) Trade(id string) *Trade {

	trade, ok := p.trades.Get(id)

	if ok {
		return trade.(*Trade)
	}

	return nil
}

func (p *Position) Trades() <-chan *Trade {

	ch := make(chan *Trade)
	go func() {
		for kv := range p.trades.Iter() {
			ch <- kv.Value.(*Trade)
		}
		close(ch)
	}()

	return ch
}

func (p *Position) TradesNumber() int32 {
	return p.tradesNumber
}

func (p *Position) Units() int32 {
	return p.units
}

func (p *Position) UnrealizedNetProfit() float64 {
	return p.unrealizedNetProfit
}

func (p *Position) UnrealizedEffectiveProfit() float64 {
	return p.unrealizedEffectiveProfit
}

func (p *Position) MarginUsed() float64 {
	return p.marginUsed
}

func (p *Position) ChargedFees() float64 {
	return p.chargedFees
}

func (p *Position) AveragePrice() float64 {
	return p.averagePrice
}
