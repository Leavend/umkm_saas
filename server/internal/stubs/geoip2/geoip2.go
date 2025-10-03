package geoip2

import (
	"fmt"
	"net"
)

// Reader is a stubbed GeoIP2 database reader used in tests.
type Reader struct{}

// Open returns an error to indicate the stub does not provide GeoIP lookups.
func Open(path string) (*Reader, error) {
	return nil, fmt.Errorf("geoip2 stub: no database support")
}

// Close implements the io.Closer interface.
func (r *Reader) Close() error {
	return nil
}

// Country is a stubbed response structure matching the real library.
type Country struct {
	Country struct {
		IsoCode string
	}
}

// Country always returns an error because the stub has no database support.
func (r *Reader) Country(ip net.IP) (*Country, error) {
	return nil, fmt.Errorf("geoip2 stub: no database support")
}
