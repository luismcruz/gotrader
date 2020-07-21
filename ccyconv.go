package gotrader

import (
	"go.uber.org/atomic"
)

type instrumentConversion struct {
	Name                    string
	BaseCurrency            string
	QuoteCurrency           string
	BaseConversionRate      *atomic.Float64
	QuoteConversionRate     *atomic.Float64
	BaseConversionFunction  []string
	QuoteConversionFunction []string
	Bid                     *atomic.Float64
	Ask                     *atomic.Float64
}

func newInstrumentConversion(name, baseCurrency, quoteCurrency string) *instrumentConversion {

	return &instrumentConversion{
		Name:                name,
		BaseCurrency:        baseCurrency,
		QuoteCurrency:       quoteCurrency,
		BaseConversionRate:  atomic.NewFloat64(0),
		QuoteConversionRate: atomic.NewFloat64(0),
		Bid:                 atomic.NewFloat64(0),
		Ask:                 atomic.NewFloat64(0),
	}
}

type currencyConversionEngine struct {
	conversionInstruments        map[string]*instrumentConversion
	conversionInstrumentsDetails []InstrumentDetails
	availableInstruments         map[string]InstrumentDetails
	conversionSet                map[string]bool            // set of instruments needed for conversion
	dependentBaseInstruments     map[string]map[string]bool // map of a set
	dependentQuoteInstruments    map[string]map[string]bool // map of a set
	homeCurrency                 string
	l                            Logger
}

func newCurrencyConversionEngine(
	tradingInsts map[string]*instrumentConversion,
	availableInsts map[string]InstrumentDetails,
	homeCcy string,
	l Logger,
) *currencyConversionEngine {

	return &currencyConversionEngine{
		conversionInstruments:        tradingInsts,
		availableInstruments:         availableInsts,
		conversionInstrumentsDetails: make([]InstrumentDetails, 0, len(tradingInsts)),
		conversionSet:                make(map[string]bool),
		dependentBaseInstruments:     make(map[string]map[string]bool),
		dependentQuoteInstruments:    make(map[string]map[string]bool),
		homeCurrency:                 homeCcy,
		l:                            l,
	}

}

func (ce *currencyConversionEngine) start() {

	for _, inst := range ce.conversionInstruments {

		ce.findBaseConversionFunctions(inst)
		ce.findQuoteConversionFunctions(inst)
		ce.conversionSet[inst.Name] = true

	}

	for id := range ce.conversionSet {

		if _, exist := ce.conversionInstruments[id]; !exist {
			ce.conversionInstruments[id] = newInstrumentConversion(
				id,
				ce.availableInstruments[id].BaseCurrency,
				ce.availableInstruments[id].QuoteCurrency,
			)
		}

		ce.conversionInstrumentsDetails = append(ce.conversionInstrumentsDetails, ce.availableInstruments[id])
	}

}

func (ce *currencyConversionEngine) setPricePointers(instruments map[string]*Instrument) {

	for _, inst := range instruments {
		ce.conversionInstruments[inst.Name()].Ask = inst.ask
		ce.conversionInstruments[inst.Name()].Bid = inst.bid
		inst.ccyConversion = ce.conversionInstruments[inst.Name()]
	}

}

func (ce *currencyConversionEngine) updateRate(instrument string) {

	for inst := range ce.dependentBaseInstruments[instrument] {
		ce.conversionInstruments[inst].BaseConversionRate.Store(ce.calculateRate(ce.conversionInstruments[inst].BaseConversionFunction))
	}

	for inst := range ce.dependentQuoteInstruments[instrument] {
		ce.conversionInstruments[inst].QuoteConversionRate.Store(ce.calculateRate(ce.conversionInstruments[inst].QuoteConversionFunction))
	}
}

func (ce *currencyConversionEngine) calculateRate(conversionFunction []string) float64 {

	result := 0.0

	for i := 0; i < len(conversionFunction)/3; i++ {

		if conversionFunction[i%3+2] == "/" {

			inst := ce.conversionInstruments[conversionFunction[i%3+1]]

			if inst.Ask == nil {
				return 0
			}

			midPrice := (inst.Bid.Load() + inst.Ask.Load()) / 2
			result = 1 / midPrice
		} else {
			inst := ce.conversionInstruments[conversionFunction[i%3+1]]

			if inst.Ask == nil {
				return 0
			}

			midPrice := (inst.Bid.Load() + inst.Ask.Load()) / 2
			result = midPrice
		}

	}

	return result

}

func (ce *currencyConversionEngine) findBaseConversionFunctions(instConv *instrumentConversion) {

	if instConv.BaseCurrency == ce.homeCurrency {

		instConv.BaseConversionRate.Store(1)

	} else if instConv.QuoteCurrency == ce.homeCurrency {

		instConv.BaseConversionFunction = []string{"1", instConv.Name, "/"}
		ce.addBaseDependentInstrument(instConv.Name, instConv.Name)

	} else {

		for id, inst := range ce.availableInstruments {

			if ce.homeCurrency == inst.QuoteCurrency && instConv.BaseCurrency == inst.BaseCurrency {
				instConv.BaseConversionFunction = []string{"1", id, "*"}
				ce.addBaseDependentInstrument(id, instConv.Name)
				ce.conversionSet[id] = true
				return
			} else if ce.homeCurrency == inst.BaseCurrency && instConv.BaseCurrency == inst.QuoteCurrency {
				instConv.BaseConversionFunction = []string{"1", id, "/"}
				ce.addBaseDependentInstrument(id, instConv.Name)
				ce.conversionSet[id] = true
				return
			}

		}

		ce.l.Warn("base conversion function for instrument " + instConv.Name + " not found")

	}

}

func (ce *currencyConversionEngine) findQuoteConversionFunctions(instConv *instrumentConversion) {

	if instConv.QuoteCurrency == ce.homeCurrency {

		instConv.BaseConversionRate.Store(1)

	} else if instConv.BaseCurrency == ce.homeCurrency {

		instConv.QuoteConversionFunction = []string{"1", instConv.Name, "/"}
		ce.addQuoteDependentInstrument(instConv.Name, instConv.Name)

	} else {

		for id, inst := range ce.availableInstruments {

			if ce.homeCurrency == inst.QuoteCurrency && instConv.QuoteCurrency == inst.BaseCurrency {
				instConv.QuoteConversionFunction = []string{"1", id, "*"}
				ce.addQuoteDependentInstrument(id, instConv.Name)
				ce.conversionSet[id] = true
				return
			} else if ce.homeCurrency == inst.BaseCurrency && instConv.QuoteCurrency == inst.QuoteCurrency {
				instConv.QuoteConversionFunction = []string{"1", id, "/"}
				ce.addQuoteDependentInstrument(id, instConv.Name)
				ce.conversionSet[id] = true
				return
			}

		}

		ce.l.Warn("quote conversion function for instrument " + instConv.Name + " not found")

	}

}

func (ce *currencyConversionEngine) addBaseDependentInstrument(parent, child string) {

	if insts := ce.dependentBaseInstruments[parent]; insts == nil {
		ce.dependentBaseInstruments[parent] = make(map[string]bool)
	}

	ce.dependentBaseInstruments[parent][child] = true

}

func (ce *currencyConversionEngine) addQuoteDependentInstrument(parent, child string) {

	if insts := ce.dependentQuoteInstruments[parent]; insts == nil {
		ce.dependentQuoteInstruments[parent] = make(map[string]bool)
	}

	ce.dependentQuoteInstruments[parent][child] = true

}
