package domain

import (
	"math"
	"testing"
)

func TestDewPointCelsius(t *testing.T) {
	t.Run("RH=100 dew point equals temperature", func(t *testing.T) {
		temp := 20.0
		td, ok := DewPointCelsius(temp, 100)
		if !ok {
			t.Fatal("expected ok=true for rh=100")
		}
		if math.Abs(td-temp) > 0.1 {
			t.Errorf("rh=100: want td≈%v, got %v", temp, td)
		}
	})

	t.Run("T=20 RH=50 dew point approx 9.3", func(t *testing.T) {
		td, ok := DewPointCelsius(20.0, 50.0)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if math.Abs(td-9.3) > 0.3 {
			t.Errorf("T=20 RH=50: want td≈9.3, got %v", td)
		}
	})

	t.Run("T=25 RH=40 dew point approx 10.5", func(t *testing.T) {
		td, ok := DewPointCelsius(25.0, 40.0)
		if !ok {
			t.Fatal("expected ok=true")
		}
		if math.Abs(td-10.5) > 0.3 {
			t.Errorf("T=25 RH=40: want td≈10.5, got %v", td)
		}
	})

	t.Run("rh=0 returns ok=false", func(t *testing.T) {
		_, ok := DewPointCelsius(20.0, 0)
		if ok {
			t.Error("expected ok=false for rh=0")
		}
	})

	t.Run("rh=120 returns ok=false", func(t *testing.T) {
		_, ok := DewPointCelsius(20.0, 120)
		if ok {
			t.Error("expected ok=false for rh=120")
		}
	})
}
