package gotrader

import (
	"sync"
)

type sortedTrades struct {
	sync.RWMutex
	tradesOrder map[string]int
	orderTrades []string
	count       int
}

func newSortedTrades() *sortedTrades {
	return &sortedTrades{
		tradesOrder: make(map[string]int),
		orderTrades: make([]string, 0, 0),
	}
}

func (cs *sortedTrades) Append(tradeID string) {
	cs.Lock()
	defer cs.Unlock()

	cs.tradesOrder[tradeID] = cs.count
	cs.orderTrades = append(cs.orderTrades, tradeID)
	cs.count++
}

func (cs *sortedTrades) AscendIter(maxIterations int) <-chan string {
	c := make(chan string)

	go func() {

		cs.RLock()

		trades := make([]string, 0, len(cs.orderTrades))

		if maxIterations == -1 {
			maxIterations = cs.count
		}

		for i := 0; (i < len(cs.orderTrades)) && (i < maxIterations); i++ {
			trades = append(trades, cs.orderTrades[i])
		}

		cs.RUnlock()

		for _, t := range trades {
			c <- t
		}

		close(c)
	}()

	return c
}

func (cs *sortedTrades) DescendIter(maxIterations int) <-chan string {
	c := make(chan string)

	go func() {
		cs.RLock()

		trades := make([]string, 0, len(cs.orderTrades))

		if maxIterations == -1 {
			maxIterations = cs.count
		}

		for i := len(cs.orderTrades) - 1; (i >= 0) && (i > -maxIterations+len(cs.orderTrades)-1); i-- {
			trades = append(trades, cs.orderTrades[i])
		}

		cs.RUnlock()

		for _, t := range trades {
			c <- t
		}

		close(c)
	}()

	return c
}

func (cs *sortedTrades) Get(index int) string {
	cs.RLock()
	defer cs.RUnlock()

	if index < -1 || index > cs.count-1 {
		return ""
	}

	if index == -1 {
		index = cs.count - 1
	}

	return cs.orderTrades[index]
}

func (cs *sortedTrades) Delete(item string) {
	cs.Lock()
	defer cs.Unlock()

	itemIndex := cs.tradesOrder[item]
	delete(cs.tradesOrder, item)
	for k, v := range cs.tradesOrder {
		if v > itemIndex {
			cs.tradesOrder[k]--
		}
	}
	cs.orderTrades = cs.orderTrades[:itemIndex+copy(cs.orderTrades[itemIndex:], cs.orderTrades[itemIndex+1:])]
	cs.count--
}

func (cs *sortedTrades) Len() int {
	cs.RLock()
	defer cs.RUnlock()

	return cs.count
}
