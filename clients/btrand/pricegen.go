package btrand

import (
	"time"

	"github.com/luismcruz/gotrader"
)

type priceGenerator struct {
	instrument string
	randGen    *randomGenerator
	price      float64
	time       time.Time
}

func newCorePriceGenerator(instrument string, startTime time.Time, startPrice float64, seed int64) *priceGenerator {

	return &priceGenerator{
		instrument: instrument,
		randGen:    newCoreRandomGenerator(seed),
		price:      startPrice,
		time:       startTime,
	}
}

func (p *priceGenerator) next() *gotrader.Tick {

	timeInc, priceInc, spread := p.randGen.next()
	p.price += priceInc

	duration := time.Duration(timeInc * float64(time.Second))

	p.time = p.time.Add(duration)

	tick := &gotrader.Tick{
		Instrument: p.instrument,
		Ask:        p.price + spread,
		Bid:        p.price,
		Time:       p.time,
	}

	return tick
}
