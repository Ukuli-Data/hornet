package dashboard

import (
	"encoding/json"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/database"
)

var (
	lastDbCleanup       = &database.DatabaseCleanup{}
	cachedDbSizeMetrics []*DBSizeMetric
)

// DBSizeMetric represents database size metrics.
type DBSizeMetric struct {
	Tangle   int64
	Snapshot int64
	Spent    int64
	Time     time.Time
}

func (s *DBSizeMetric) MarshalJSON() ([]byte, error) {
	size := struct {
		Tangle   int64 `json:"tangle"`
		Snapshot int64 `json:"snapshot"`
		Spent    int64 `json:"spent"`
		Time     int64 `json:"ts"`
	}{
		Tangle:   s.Tangle,
		Snapshot: s.Snapshot,
		Spent:    s.Spent,
		Time:     s.Time.Unix(),
	}

	return json.Marshal(size)
}

func currentDatabaseSize() *DBSizeMetric {
	tangle, snapshot, spent := tangle.GetDatabaseSizes()
	newValue := &DBSizeMetric{
		Tangle:   tangle,
		Snapshot: snapshot,
		Spent:    spent,
		Time:     time.Now(),
	}
	cachedDbSizeMetrics = append(cachedDbSizeMetrics, newValue)
	if len(cachedDbSizeMetrics) > 600 {
		cachedDbSizeMetrics = cachedDbSizeMetrics[len(cachedDbSizeMetrics)-600:]
	}
	return newValue
}

func runDatabaseSizeCollector() {

	// Gather first metric so we have a starting point
	currentDatabaseSize()

	notifyDatabaseCleanup := events.NewClosure(func(cleanup *database.DatabaseCleanup) {
		lastDbCleanup = cleanup
		wsSendWorkerPool.TrySubmit(cleanup)
	})

	daemon.BackgroundWorker("Dashboard[DBSize]", func(shutdownSignal <-chan struct{}) {
		database.Events.DatabaseCleanup.Attach(notifyDatabaseCleanup)
		timeutil.Ticker(func() {
			dbSizeMetric := currentDatabaseSize()
			wsSendWorkerPool.TrySubmit([]*DBSizeMetric{dbSizeMetric})
		}, 1*time.Minute, shutdownSignal)
		database.Events.DatabaseCleanup.Detach(notifyDatabaseCleanup)
	}, shutdown.PriorityDashboard)
}
