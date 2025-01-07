package prometheus_test

import (
	"context"
	"testing"
	"time"

	"github.com/equinor/radix-api/api/metrics/prometheus"
	mock2 "github.com/equinor/radix-api/api/metrics/prometheus/mock"
	"github.com/golang/mock/gomock"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

func TestArguemtsExistsInQuery(t *testing.T) {

	ctrl := gomock.NewController(t)
	mock := mock2.NewMockQueryAPI(ctrl)

	gomock.InOrder(
		mock.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {

				assert.Contains(t, query, "namespace1")
				assert.Contains(t, query, "app1")

				return nil, nil, nil
			},
		),
		mock.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {

				assert.Contains(t, query, "namespace2")
				assert.Contains(t, query, "app2")

				return nil, nil, nil
			},
		),
		mock.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {

				assert.Contains(t, query, "namespace3")
				assert.Contains(t, query, "app3")
				assert.Contains(t, query, "24h")

				return nil, nil, nil
			},
		),
		mock.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {

				assert.Contains(t, query, "namespace4")
				assert.Contains(t, query, "app4")
				assert.Contains(t, query, "36h")

				return nil, nil, nil
			},
		),
	)

	client := prometheus.NewClient(mock)
	_, _ = client.GetCpuReqs(context.Background(), "app1", "namespace1")
	_, _ = client.GetMemReqs(context.Background(), "app2", "namespace2")
	_, _ = client.GetCpuAvg(context.Background(), "app3", "namespace3", "24h")
	_, _ = client.GetMemMax(context.Background(), "app4", "namespace4", "36h")
}
