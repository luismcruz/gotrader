package gotrader

import (
	"errors"
)

// Option represents trading session functional option
type Option func(p *sessionParameters)

// Instruments is the functional option to define the instruments to trade
func Instruments(instruments []string) Option {
	return func(p *sessionParameters) {
		p.instruments = instruments
	}
}

// AccountID is the functional option to define the account where the session will operate
func AccountID(account string) Option {
	return func(p *sessionParameters) {
		p.account = account
	}
}

// HedgeType is the functional option to choose how hedge is calculated in the backtest engine.
func HedgeType(hedge Hedge) Option {
	return func(p *sessionParameters) {
		if p.testParameters != nil {
			p.testParameters.hedge = hedge
		} else {
			p.testParameters = &testParameters{
				hedge: hedge,
			}
		}
	}
}

// InitialBalance is the functional option to define the initial balance in the backtest engine.
func InitialBalance(value float64) Option {
	return func(p *sessionParameters) {
		if p.testParameters != nil {
			p.testParameters.initialBalance = value
		} else {
			p.testParameters = &testParameters{
				initialBalance: value,
			}
		}
	}
}

// HomeCurrency is the functional option to define home currency in the backtest engine.
func HomeCurrency(ccy string) Option {

	return func(p *sessionParameters) {
		if p.testParameters != nil {
			p.testParameters.homeCurrency = ccy
		} else {
			p.testParameters = &testParameters{
				homeCurrency: ccy,
			}
		}
	}
}

// Leverage is the functional option to define account leverage in the backtest engine.
func Leverage(leverage float64) Option {

	return func(p *sessionParameters) {
		if p.testParameters != nil {
			p.testParameters.leverage = leverage
		} else {
			p.testParameters = &testParameters{
				leverage: leverage,
			}
		}
	}
}

type testParameters struct {
	initialBalance float64
	homeCurrency   string
	leverage       float64
	hedge          Hedge
}

type sessionParameters struct {
	instruments    []string
	account        string
	testParameters *testParameters
}

// TradingSession represents the entrypoint struct of the gotrader package, representing a trading session.
type TradingSession struct {
	strategy   Strategy
	engine     Engine
	client     BrokerClient
	parameters *sessionParameters
	engineType int
}

// NewTradingSession is the TradingSession constructor.
func NewTradingSession(opts ...Option) *TradingSession {

	params := &sessionParameters{}

	for _, o := range opts {
		o(params)
	}

	return &TradingSession{
		parameters: params,
	}
}

// SetStrategy sets the strategy to trade.
func (s *TradingSession) SetStrategy(strategy Strategy) *TradingSession {
	s.strategy = strategy

	return s
}

// SetClient sets the client that will be used for comunication with the broker.
func (s *TradingSession) SetClient(client BrokerClient) *TradingSession {
	s.client = client

	return s
}

// Live defines that this is a live session.
func (s *TradingSession) Live() *TradingSession {
	s.engine = newLiveEngine()

	return s
}

// Backtest defines that this is a backtesting session.
func (s *TradingSession) Backtest() *TradingSession {
	s.engine = newBtEngine()
	s.engineType = 1

	return s
}

// Start trading session.
func (s *TradingSession) Start() error {

	if s.engine == nil {
		return errors.New("engine type is not defined")
	}

	var err error

	switch s.engineType {
	case 0:
		engine := s.engine.(*liveEngine)
		engine.client = s.client
		engine.strategy = s.strategy
		engine.parameters = s.parameters
		err = engine.start()
	case 1:
		engine := s.engine.(*btEngine)
		engine.client = s.client
		engine.strategy = s.strategy
		engine.parameters = s.parameters
		err = engine.start()
	}

	return err
}
