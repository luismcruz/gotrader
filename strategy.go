package gotrader

// Strategy is the interface that a strategy must implement in order to be used by this engine.
type Strategy interface {
	Initialize()
	SetEngine(engine Engine)
	OnOrderFill(orderFill *OrderFill)
	OnTick(tick *Tick)
	OnStop()
}
