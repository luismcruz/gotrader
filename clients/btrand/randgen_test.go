package btrand

import (
	"fmt"
	"os"
	"testing"
)

func Test_newCoreRandomGenerator(t *testing.T) {

	t.Run("test1", func(t *testing.T) {

		gen := newCoreRandomGenerator(5978461448311832750)

		points := 500000 * 6

		price := 1.48
		time := 0.0

		/*priceVec := make([]float64, points, points)
		timeVec := make([]float64, points, points)*/

		f, err := os.Create("./datapoints.csv")
		if err != nil {
			return
		}
		defer f.Close()

		for i := 0; i < points; i++ {

			timeInc, priceInc, _ := gen.next()

			time += timeInc
			price += priceInc

			fmt.Fprintln(f, time, ",", price)
		}

	})

}
