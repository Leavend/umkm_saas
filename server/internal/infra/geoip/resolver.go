package geoip

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

// ErrUnavailable is returned when the resolver is not initialized.
var ErrUnavailable = errors.New("geoip resolver unavailable")

// CountryResolver resolves ISO country codes from IP addresses.
type CountryResolver interface {
	CountryCode(ip string) (string, error)
}

// Resolver provides country lookups backed by a MaxMind GeoIP2 database.
type Resolver struct {
	reader *geoip2.Reader
}

// NewResolver opens the GeoIP database at the given path. When the path is empty, nil is returned.
func NewResolver(path string) (CountryResolver, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	reader, err := geoip2.Open(path)
	if err != nil {
		return nil, fmt.Errorf("geoip: open database: %w", err)
	}
	return &Resolver{reader: reader}, nil
}

// CountryCode returns the ISO country code for the provided IP.
func (r *Resolver) CountryCode(ip string) (string, error) {
	if r == nil || r.reader == nil {
		return "", ErrUnavailable
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", fmt.Errorf("geoip: invalid ip %q", ip)
	}
	record, err := r.reader.Country(parsed)
	if err != nil {
		return "", fmt.Errorf("geoip: lookup country: %w", err)
	}
	if record == nil || record.Country.IsoCode == "" {
		return "", nil
	}
	return record.Country.IsoCode, nil
}

// Close closes the underlying database reader.
func (r *Resolver) Close() error {
	if r == nil || r.reader == nil {
		return nil
	}
	return r.reader.Close()
}
