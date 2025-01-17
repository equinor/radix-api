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

				return nil, nil, nil
			},
		),
		mock.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {

				assert.Contains(t, query, "namespace2")

				return nil, nil, nil
			},
		),
		mock.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {

				assert.Contains(t, query, "namespace3")
				assert.Contains(t, query, "24h")

				return nil, nil, nil
			},
		),
		mock.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {

				assert.Contains(t, query, "namespace4")
				assert.Contains(t, query, "36h")

				return nil, nil, nil
			},
		),
	)

	client := prometheus.NewClient(mock)
	_, _ = client.GetCpuRequests(context.Background(), "namespace1")
	_, _ = client.GetMemoryRequests(context.Background(), "namespace2")
	_, _ = client.GetCpuAverage(context.Background(), "namespace3", "24h")
	_, _ = client.GetMemoryMaximum(context.Background(), "namespace4", "36h")
}
