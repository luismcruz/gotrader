package btrand

import (
	"math/rand"
)

/************************************************************
Core probabilities/rates/stds/averages of the generator
Mu -> average
Sigma -> standard deviation
Prob -> probabilities
*************************************************************/
const (
	timePaceRateCore          float64 = 0.5
	noiseSigmaCore            float64 = 0.0000005
	trendChangeProbCore       float64 = 0.20
	trendMuCore               float64 = 0.0000005
	burstActivationProbCore   float64 = 0.10
	burstDeactivationProbCore float64 = 0.90
	burstSigmaCore            float64 = 0.00005
	spreadMinCore             float64 = 0.00005
	spreadMaxCore             float64 = 0.00025
)

/*

Random generator with time pace and 3 components noise, trend and volatility bursts

*/
type randomGenerator struct {
	timePaceRate      float64
	noiseSigma        float64
	trendChange       float64
	trendMu           float64
	burstActivation   float64
	burstDeactivation float64
	burstSigma        float64
	burstActivated    bool
	spreadMin         float64
	spreadMax         float64
	rand              *rand.Rand
}

type Option func(g *randomGenerator)

func TimePaceRate(p float64) Option {
	return func(g *randomGenerator) {
		g.timePaceRate = p
	}
}

func NoiseSigma(p float64) Option {
	return func(g *randomGenerator) {
		g.noiseSigma = p
	}
}

func newCoreRandomGenerator(seed int64) *randomGenerator {

	gen := &randomGenerator{
		timePaceRate:      timePaceRateCore,
		noiseSigma:        noiseSigmaCore,
		trendChange:       trendChangeProbCore,
		trendMu:           trendMuCore,
		burstActivation:   burstActivationProbCore,
		burstDeactivation: burstDeactivationProbCore,
		burstSigma:        burstSigmaCore,
		spreadMin:         spreadMinCore,
		spreadMax:         spreadMaxCore,
		rand:              rand.New(rand.NewSource(seed)),
	}

	gen.trendMu = gen.trendMu * float64(gen.rand.Int63n(2)*2-1)

	return gen
}

func newRandomGenerator(seed int64, opts ...Option) *randomGenerator {

	gen := newCoreRandomGenerator(seed)

	for _, o := range opts {
		o(gen)
	}

	return gen
}

func (g *randomGenerator) next() (float64, float64, float64) {

	timeInc := g.rand.Float64() * g.timePaceRate

	price := g.rand.NormFloat64()*g.noiseSigma + g.trendMu

	if g.rand.Float64() < g.trendChange {
		g.trendMu = -g.trendMu
	}

	if !g.burstActivated && g.rand.Float64() < g.burstActivation {
		g.burstActivated = true
	}

	if g.burstActivated {
		price += g.rand.NormFloat64() * g.burstSigma
	}

	if g.burstActivated && g.rand.Float64() < g.burstDeactivation {
		g.burstActivated = false
	}

	spread := g.rand.Float64()*(g.spreadMax-g.spreadMin) + g.spreadMin

	return timeInc, price, spread
}
