package main


import (
	"testing"

	"github.com/bloomberg/go-testgroup"
)

type CalcGroup struct{}

func (g *CalcGroup) Addition(t *testgroup.T) {
	t.Run("1+1=2", func(t *testgroup.T) { t.Equal(2, 1+1) })
	t.Run("2+3=5", func(t *testgroup.T) { t.Equal(5, 2+3) })
}

func (g *CalcGroup) Multiplication(t *testgroup.T) {
	t.Run("2*2=4", func(t *testgroup.T) { t.Equal(4, 2*2) })
	t.Run("3*3=9", func(t *testgroup.T) { t.Equal(9, 3*3) })
}

func TestCalcSerial(t *testing.T)   { testgroup.RunSerially(t, &CalcGroup{}) }

// Or run in parallel.
// Don't call t.Parallel inside methods
func TestCalcParallel(t *testing.T) { testgroup.RunInParallel(t, &CalcGroup{}) }
