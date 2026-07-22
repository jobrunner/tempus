package domain

// Provider status values reported in QueryResult.Providers.
const (
	StatusOK          = "ok"
	StatusUnavailable = "unavailable"
	StatusError       = "error"
)

// License is the attribution block attached to every feature. All three fields
// are required — a feature without attribution is a contract violation.
type License struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Attribution string `json:"attribution"`
}

// Coordinate is a WGS84 point.
type Coordinate struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

// Geometry is a GeoJSON geometry (only Point is produced today).
type Geometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"` // [lon, lat]
}

// Feature is a GeoJSON-style feature returned by a provider.
type Feature struct {
	Type       string         `json:"type"`
	Geometry   Geometry       `json:"geometry"`
	Properties map[string]any `json:"properties"`
	License    License        `json:"license"`
}

// NewPointFeature builds a Point Feature at coord with the given properties and
// license. coord here is the provider-resolved location (e.g. grid cell).
func NewPointFeature(coord Coordinate, props map[string]any, lic License) Feature {
	return Feature{
		Type:       "Feature",
		Geometry:   Geometry{Type: "Point", Coordinates: []float64{coord.Lon, coord.Lat}},
		Properties: props,
		License:    lic,
	}
}

// ProviderResult is what a FeatureProvider returns: the feature plus whether it
// was served from cache (set by the caching decorator, false otherwise).
type ProviderResult struct {
	Feature Feature
	Cached  bool
}

// ProviderStatus reports the outcome of one provider in the response envelope.
type ProviderStatus struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	Cached     bool   `json:"cached,omitempty"`
	Retryable  bool   `json:"retryable"`
	RetryAfter string `json:"retryAfter,omitempty"`
	Error      string `json:"error,omitempty"`
}

// QueryEcho echoes the resolved request (makes the idempotent contract explicit).
type QueryEcho struct {
	Coordinate Coordinate `json:"coordinate"`
	Datetime   string     `json:"datetime"`
}

// QueryResult is the assembled response payload.
type QueryResult struct {
	Query     QueryEcho        `json:"query"`
	Features  []Feature        `json:"features"`
	Providers []ProviderStatus `json:"providers"`
}
