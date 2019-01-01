package gotrader

import (
	"time"

	"go.uber.org/atomic"
)

// Account represent the current account status. Mirrors the broker status.
type Account struct {
	id                        string
	instruments               map[string]*Instrument
	time                      time.Time
	homeCurrency              string
	equity                    float64
	balance                   *atomic.Float64
	unrealizedNetProfit       float64
	unrealizedEffectiveProfit float64
	chargedFees               float64
	marginUsed                float64
	marginFree                float64
	leverage                  float64
}

/**************************
*
*	Internal Methods
*
***************************/

func newAccount(accountID string) *Account {

	return &Account{
		id:          accountID,
		instruments: make(map[string]*Instrument),
		balance:     atomic.NewFloat64(0.0),
	}

}

func (a *Account) calculateUnrealized() {

	unrealizedNet := 0.0
	unrealizedEffective := 0.0
	chargedFees := 0.0

	for _, instrument := range a.instruments {

		instrument.calculateUnrealized()

		unrealizedNet += instrument.unrealizedNetProfit
		unrealizedEffective += instrument.unrealizedEffectiveProfit
		chargedFees += instrument.chargedFees
	}

	a.unrealizedNetProfit = unrealizedNet
	a.unrealizedEffectiveProfit = unrealizedEffective
	a.equity = a.unrealizedNetProfit + a.balance.Load()
	a.chargedFees = chargedFees
}

func (a *Account) calculateMarginUsed() {

	marginUsed := 0.0

	for _, instrument := range a.instruments {

		instrument.calculateMarginUsed()
		marginUsed += instrument.marginUsed

	}

	a.marginUsed = marginUsed

}

func (a *Account) calculateFreeMargin() {
	a.marginFree = a.equity - a.marginUsed
}

/**************************
*
*	Accessible Methods
*
***************************/

func (a *Account) ID() string {
	return a.id
}

func (a *Account) Instruments() map[string]*Instrument {
	return a.instruments
}

func (a *Account) Instrument(instrument string) *Instrument {
	return a.instruments[instrument]
}

func (a *Account) HomeCurrency() string {
	return a.homeCurrency
}

func (a *Account) Equity() float64 {
	return a.equity
}

func (a *Account) Balance() float64 {
	return a.balance.Load()
}

func (a *Account) UnrealizedNetProfit() float64 {
	return a.unrealizedNetProfit
}

func (a *Account) UnrealizedEffectiveProfit() float64 {
	return a.unrealizedEffectiveProfit
}

func (a *Account) ChargedFees() float64 {
	return a.chargedFees
}

func (a *Account) MarginUsed() float64 {
	return a.marginUsed
}

func (a *Account) MarginFree() float64 {
	return a.marginFree
}

func (a *Account) Time() time.Time {
	return a.time
}
