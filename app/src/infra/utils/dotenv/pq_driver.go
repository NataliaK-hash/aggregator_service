package dotenv

import (
	"database/sql"
	"sync"

	"github.com/lib/pq"
)

var registerAggregatorDriverOnce sync.Once

func init() {
	registerAggregatorDriverOnce.Do(func() {
		sql.Register("aggregator", &pq.Driver{})
	})
}
