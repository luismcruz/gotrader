# gotrader

## Overview

A golang package to automatize trading strategies. This package was mainly developed to trade FOREX, and it depends on fact that the broker should provide control over trades (single transactions). Examples of such brokers are Oanda and IG if using rest API's. In the other hand, the FIX protocol only allow to close or reduce positions (defined here as an aggregation of trades) which is not supported by the package yet.

## Usage

Create a streategy that implements the Strategy interface:

```Go
type MyStrategy struct {
    engine gotrader.Engine
}

func (s *MyStrategy) SetEngine(engine gotrader.Engine) { // Engine will inject itself to the strategy
    s.engine = engine
}

func (s *MyStrategy) OnTick(tick *gotrader.Tick) {
    // Main Strategy logic
}

...

```

Then just instantiate a trading session with the strategy and client you need:

```Go
strategy := &MyStrategy{}

client := oanda.NewOandaClient("my-token", false)

session := gotrader.NewTradingSession(
		gotrader.Instruments([]string{
			"EUR_USD",
		}),
		gotrader.AccountID("my-account"),
)

session.SetStrategy(strategy).SetClient(client).Live()

if err := session.Start(); err != nil {
    logrus.Warn(err)
}

```

## Included Clients

Oanda client and random generator for testing.

## TODO

- Write tests for gotrader
- Include more broker clients
- Refactor to add the possibility to use FIX protocol on the clients (control over position only)
- Refactor to include pluggable metrics, such candlesticks, moving averages or user customized metrics. 
