package bioclim

import (
	"fmt"
	"net/url"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
)

// retryFallback is the retry hint for a throttled or failing upstream that
// sends no usable Retry-After header. It is twice the other Open-Meteo
// adapters' 30s: the 30-year daily archive range is the heaviest call tempus
// makes (Open-Meteo counts it as many calls), so backing off longer before
// asking again is kinder to the shared rate limit and daily budget.
const retryFallback = 60 * time.Second

// buildURL builds the ERA5 archive request URL for the given coordinate and
// reference-period year range.
func (p *Provider) buildURL(coord domain.Coordinate, startY, endY int) (string, error) {
	u, err := url.Parse(p.archiveBaseURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("latitude", fmt.Sprintf("%.5f", coord.Lat))
	q.Set("longitude", fmt.Sprintf("%.5f", coord.Lon))
	q.Set("timezone", "UTC")
	q.Set("daily", "temperature_2m_max,temperature_2m_min,temperature_2m_mean,precipitation_sum")
	q.Set("start_date", fmt.Sprintf("%d-01-01", startY))
	q.Set("end_date", fmt.Sprintf("%d-12-31", endY))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (p *Provider) license(period string) domain.License {
	return domain.License{
		Name:        licenseName,
		URL:         licenseURL,
		Attribution: "Weather data by Open-Meteo.com; ERA5 (Copernicus Climate Change Service / ECMWF), " + period + "; " + domain.BioclimSource,
	}
}
