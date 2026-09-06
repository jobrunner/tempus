package bioclim

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/jobrunner/tempus/internal/domain"
	"github.com/jobrunner/tempus/internal/ports/output"
)

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

func (p *Provider) getJSON(ctx context.Context, u string, dst any) error {
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		if pe, ok := output.AsProviderError(err); ok {
			return pe
		}
		return output.NewTransientError(err, 30*time.Second)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return output.NewTransientError(fmt.Errorf("open-meteo status %d", resp.StatusCode), retryAfter(resp))
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return output.NewPermanentError(fmt.Errorf("open-meteo status %d: %s", resp.StatusCode, b))
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return output.NewPermanentError(err)
	}
	return nil
}

// retryAfter reads the response's Retry-After header (seconds) so the
// envelope's hint reflects what Open-Meteo actually asked for; it falls back
// to a fixed 60s when the header is absent or unparsable.
func retryAfter(resp *http.Response) time.Duration {
	if secs, err := time.ParseDuration(resp.Header.Get("Retry-After") + "s"); err == nil {
		return secs
	}
	return 60 * time.Second
}

func (p *Provider) license(period string) domain.License {
	return domain.License{
		Name:        licenseName,
		URL:         licenseURL,
		Attribution: "Weather data by Open-Meteo.com; ERA5 (Copernicus Climate Change Service / ECMWF), " + period + "; " + domain.BioclimSource,
	}
}
