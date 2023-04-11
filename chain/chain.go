package chain

import (
	"errors"
	"log"
	"net/http"
	"net/url"

	"github.com/djhworld/theunwrapper/unwrap"
)

var (
	ErrNoUnwrapperFound = errors.New("no unwrapper found")
)

func New(r *http.Request, unwrappers map[string]*unwrap.Unwrapper) (*ChainedUnwrapper, error) {
	var start *unwrap.Unwrapper
	if host := r.Header.Get("X-Forwarded-Host"); host != "" {
		start = unwrappers[host]
		if start == nil {
			return nil, ErrNoUnwrapperFound
		} else {
			log.Printf("using unwrapper for: %s", start.Host())
		}
	} else {
		return nil, ErrNoUnwrapperFound
	}

	return &ChainedUnwrapper{
		ur:         r.URL,
		unwrapper:  start,
		chain:      []Entry{},
		unwrappers: unwrappers,
		visitList:  make(map[string]struct{}),
	}, nil
}

// Entry describes the transition from moving to one URL to the next, given
// the unwrapper that was used
type Entry struct {
	From  url.URL
	To    url.URL
	Using unwrap.Unwrapper
}

type ChainedUnwrapper struct {
	ur         *url.URL
	unwrapper  *unwrap.Unwrapper
	chain      []Entry
	visitList  map[string]struct{}
	unwrappers map[string]*unwrap.Unwrapper
	err        error
}

// Err returns the last error set
func (c *ChainedUnwrapper) Err() error {
	return c.err
}

// Last returns the currently set URL
func (c *ChainedUnwrapper) Last() *url.URL {
	return c.ur
}

// Visited returns a slice of ChainEntry structs that describe the
// hops visited before finding the final URL
func (c *ChainedUnwrapper) Visited() []Entry {
	return c.chain
}

// Next will visit the next endpoint in the chain.
// Returns false when the end of the chain is reached or if there is an error
func (c *ChainedUnwrapper) Next() bool {
	// try to ensure we don't visit the same URL twice
	if _, ok := c.visitList[c.ur.String()]; ok {
		log.Printf("error: cycle detected!")
		c.err = errors.New("cycle detetected")
		return false
	}

	endpoint, result, err := c.unwrapper.Do(c.ur.Path[1:])
	if err != nil {
		c.err = err
		return false
	}
	c.visitList[endpoint.String()] = struct{}{}

	if result != nil {
		c.chain = append(c.chain, Entry{From: *endpoint, To: *result, Using: *c.unwrapper})
		if r, ok := c.unwrappers[result.Host]; ok {
			c.unwrapper = r
			c.ur = result
			return true
		}
	} else {
		log.Printf("error: failed to lookup!")
		c.unwrapper = nil
		return false
	}

	log.Printf("finished, found: %s", result)

	c.unwrapper = nil
	c.ur = result
	return false
}
