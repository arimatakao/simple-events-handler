package aggregator

import (
	"fmt"
	"os"
	"strconv"

	"log/slog"

	"github.com/arimatakao/simple-events-handler/internal/database"
	"github.com/robfig/cron/v3"
)

// Aggregator manages a cron scheduler that periodically calls db.AggregateEvents.
type Aggregator struct {
	c              *cron.Cron
	entryID        cron.EntryID
	db             database.Aggregatter
	logger         *slog.Logger
	intervalSecond int
}

func New(logger *slog.Logger) (*Aggregator, error) {
	aggSeconds := 60
	if s := os.Getenv("AGGREGATION_INTERVAL_SECONDS"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			if v <= 0 {
				return nil, fmt.Errorf("invalid range number of AGGREGATION_INTERVAL_SECONDS=%s: must be positive integer", s)
			}
			aggSeconds = v
		} else {
			logger.Warn("invalid AGGREGATION_INTERVAL_SECONDS, using default 60 seconds", "error", err.Error())
		}
	}

	db := database.New()

	c := cron.New(cron.WithSeconds())
	spec := "@every " + strconv.Itoa(aggSeconds) + "s"
	id, err := c.AddFunc(spec, func() {
		logger.Info("Aggregation started")
		if err := db.AggregateEvents(aggSeconds); err != nil {
			logger.Error("aggregation error", "error", err.Error())
		} else {
			logger.Info("Aggregation completed successfully")
		}
	})
	if err != nil {
		return nil, err
	}

	return &Aggregator{
		c:              c,
		entryID:        id,
		db:             db,
		logger:         logger,
		intervalSecond: aggSeconds,
	}, nil
}

// Start begins the scheduled aggregation job. It is safe to call Start multiple times.
func (a *Aggregator) Start() error {
	a.c.Start()
	a.logger.Info("aggregation cron started", "interval_seconds", a.intervalSecond)
	return nil
}

// Stop stops the cron scheduler.
func (a *Aggregator) Stop() {
	if a.c != nil {
		a.c.Stop()
		a.logger.Info("aggregation cron stopped", "cron_entry_id", a.entryID)
	}
}
