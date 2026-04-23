package concurrent_aggregator

import (
	"context"
	"fmt"
	"github.com/medunes/go-kata/01-context-cancellation-concurrency/01-concurrent-aggregator/order"
	"github.com/medunes/go-kata/01-context-cancellation-concurrency/01-concurrent-aggregator/profile"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"time"
)

type UserAggregator struct {
	// По значению, так как реализация по значению
	profileService profile.Service
	orderService   order.Service
	logger         slog.Logger   // Обычно по значению и заполнение default значением в случае если не передано. Иначе везде придется проверять на nil.
	timeout        time.Duration // По значению и проверка на 0
}

func NewUserAggregator(orderService order.Service, profileService profile.Service, options ...OptFunc) *UserAggregator {
	agg := &UserAggregator{
		orderService:   orderService,
		profileService: profileService,
		logger:         *slog.Default(),
	}
	for _, opt := range options {
		opt(agg)
	}
	return agg
}

type OptFunc func(agg *UserAggregator)

func WithTimeout(timeout time.Duration) OptFunc {
	return func(agg *UserAggregator) {
		agg.timeout = timeout
	}
}

func WithLogger(logger *slog.Logger) OptFunc {
	return func(agg *UserAggregator) {
		agg.logger = *logger
	}
}

type AggregatedProfile struct {
	Name string
	Cost float64
}

func (ua *UserAggregator) Aggregate(ctx context.Context, userId int) ([]*AggregatedProfile, error) {
	g, ctx := errgroup.WithContext(ctx)

	var pr *profile.Profile
	var orders []*order.Order
	var cancel context.CancelFunc

	if ua.timeout != 0 {
		ctx, cancel = context.WithTimeout(ctx, ua.timeout)
		defer cancel()
	}

	ua.logger.Info("starting aggregation", "user_id", userId)
	g.Go(func() error {
		var err error
		pr, err = ua.profileService.Get(ctx, userId)
		if err != nil {
			ua.logger.Warn("failed fetch profile", "user_id", userId, "err", err)
			return fmt.Errorf("pr get: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		var err error
		orders, err = ua.orderService.GetAll(ctx, userId)
		if err != nil {
			ua.logger.Warn("failed fetch orders", "user_id", userId, "err", err)
			return fmt.Errorf("orders get all: %w", err)
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		ua.logger.Warn("aggregator exited with error", "user_id", userId, "err", err)
		return nil, err
	}

	var res []*AggregatedProfile
	for _, o := range orders {
		if o.UserId == userId {
			res = append(res, &AggregatedProfile{Name: pr.Name, Cost: o.Cost})
		}
	}
	ua.logger.Info("aggregation complete successfully", "user_id", userId, "count", len(res))

	return res, nil
}
