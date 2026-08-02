package domain

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// BioclimSource is the attribution for the bioclim feature.
const BioclimSource = "BIO-Variablen (WorldClim-Definitionen) und Köppen-Geiger, berechnet aus ERA5 von tempus"

// era5StartYear is the first year ERA5 (Open-Meteo archive) provides. Reference
// periods are clamped to start no earlier than this.
const era5StartYear = 1940

// normalWidth is the length of a climate-normal period in years.
const normalWidth = 30

// NormalPeriod returns the [startYear, endYear] climate-normal reference period.
// If override is non-empty it must be "YYYY-YYYY"; otherwise the contemporaneous
// 30-year WMO standard normal containing the instant's year is used (1931-1960,
// 1961-1990, 1991-2020, …), clamped so the start is not before ERA5 (1940).
func NormalPeriod(instant time.Time, override string) (startYear, endYear int, err error) {
	if strings.TrimSpace(override) != "" {
		return parseRefPeriod(override)
	}
	y := instant.UTC().Year()
	switch {
	case y >= 1991:
		startYear = 1991
	case y >= 1961:
		startYear = 1961
	default:
		startYear = 1931
	}
	endYear = startYear + normalWidth - 1
	if startYear < era5StartYear {
		startYear = era5StartYear
		endYear = startYear + normalWidth - 1
	}
	return startYear, endYear, nil
}

// parseRefPeriod parses and validates a "YYYY-YYYY" override.
func parseRefPeriod(s string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(s), "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("refPeriod must be YYYY-YYYY, got %q", s)
	}
	start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || start < era5StartYear || end <= start || end-start > 100 {
		return 0, 0, fmt.Errorf("refPeriod must be YYYY-YYYY with start ≥ %d and end > start, got %q", era5StartYear, s)
	}
	return start, end, nil
}

// MonthlyClimate holds the 12-month (Jan…Dec) climatological normals used to
// compute the bioclimatic variables and the Köppen-Geiger class. Temperatures
// are °C; precipitation is mm (monthly totals).
type MonthlyClimate struct {
	Tmin   [12]float64
	Tmax   [12]float64
	Tmean  [12]float64
	Precip [12]float64
}

// Bioclim holds the 19 WorldClim bioclimatic variables. Temperatures are °C,
// precipitation mm; BIO3/BIO4/BIO15 are the conventional derived indices.
type Bioclim struct {
	Bio1, Bio2, Bio3, Bio4, Bio5, Bio6, Bio7, Bio8, Bio9, Bio10   float64
	Bio11, Bio12, Bio13, Bio14, Bio15, Bio16, Bio17, Bio18, Bio19 float64
}

// BioclimVariables computes BIO1–BIO19 from the monthly climate normals, using
// the standard WorldClim/ANUCLIM definitions. Quarters are the 12 overlapping
// runs of three consecutive months (wrapping around the year).
func BioclimVariables(c MonthlyClimate) Bioclim {
	var b Bioclim

	// Quarterly aggregates (index i = quarter starting in month i).
	var qTemp, qPrec [12]float64
	for i := 0; i < 12; i++ {
		qTemp[i] = (c.Tmean[i] + c.Tmean[(i+1)%12] + c.Tmean[(i+2)%12]) / 3
		qPrec[i] = c.Precip[i] + c.Precip[(i+1)%12] + c.Precip[(i+2)%12]
	}

	b.Bio1 = mean12(c.Tmean)
	var diurnal [12]float64
	for i := 0; i < 12; i++ {
		diurnal[i] = c.Tmax[i] - c.Tmin[i]
	}
	b.Bio2 = mean12(diurnal)
	b.Bio5 = max12(c.Tmax)
	b.Bio6 = min12(c.Tmin)
	b.Bio7 = b.Bio5 - b.Bio6
	if b.Bio7 != 0 {
		b.Bio3 = b.Bio2 / b.Bio7 * 100
	}
	b.Bio4 = stddevPop12(c.Tmean) * 100

	warmQ, coldQ := argMax12(qTemp), argMin12(qTemp)
	wetQ, dryQ := argMax12(qPrec), argMin12(qPrec)
	b.Bio8 = qTemp[wetQ]
	b.Bio9 = qTemp[dryQ]
	b.Bio10 = qTemp[warmQ]
	b.Bio11 = qTemp[coldQ]

	b.Bio12 = sum12(c.Precip)
	b.Bio13 = max12(c.Precip)
	b.Bio14 = min12(c.Precip)
	if m := mean12(c.Precip); m != 0 {
		b.Bio15 = stddevPop12(c.Precip) / m * 100
	}
	b.Bio16 = qPrec[wetQ]
	b.Bio17 = qPrec[dryQ]
	b.Bio18 = qPrec[warmQ]
	b.Bio19 = qPrec[coldQ]
	return b
}

func mean12(x [12]float64) float64 { return sum12(x) / 12 }

func sum12(x [12]float64) float64 {
	var s float64
	for _, v := range x {
		s += v
	}
	return s
}

func max12(x [12]float64) float64 {
	m := x[0]
	for _, v := range x {
		if v > m {
			m = v
		}
	}
	return m
}

func min12(x [12]float64) float64 {
	m := x[0]
	for _, v := range x {
		if v < m {
			m = v
		}
	}
	return m
}

func argMax12(x [12]float64) int {
	idx := 0
	for i, v := range x {
		if v > x[idx] {
			idx = i
		}
	}
	return idx
}

func argMin12(x [12]float64) int {
	idx := 0
	for i, v := range x {
		if v < x[idx] {
			idx = i
		}
	}
	return idx
}

func stddevPop12(x [12]float64) float64 {
	m := mean12(x)
	var s float64
	for _, v := range x {
		s += (v - m) * (v - m)
	}
	return math.Sqrt(s / 12)
}
