package smtp_stats_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mailvault/mailvault/domain/entities"
	"github.com/mailvault/mailvault/domain/smtp_stats"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRepo is an in-test stub for smtp_stats.Repository — moq is not generated
// for this interface, and the tests only need a handful of method captures.
type stubRepo struct {
	createStat              func(ctx context.Context, stat *entities.SMTPVerificationStat) error
	getStatsForDomain       func(ctx context.Context, domainID uuid.UUID, filter entities.SMTPStatsFilter, limit, offset int) ([]entities.SMTPVerificationStat, int64, error)
	getStatsForEmailAddress func(ctx context.Context, emailAddressID uuid.UUID, filter entities.SMTPStatsFilter, limit, offset int) ([]entities.SMTPVerificationStat, int64, error)
	getOverview             func(ctx context.Context, filter entities.SMTPStatsFilter) (*entities.SMTPStatsOverview, error)
	getTimeSeriesData       func(ctx context.Context, filter entities.SMTPStatsFilter, granularity string) ([]entities.TimeSeriesPoint, error)
	getActionDistribution   func(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.ActionDistribution, error)
	deleteOldStats          func(ctx context.Context, olderThan time.Duration) (int64, error)
}

func (s *stubRepo) CreateStat(ctx context.Context, stat *entities.SMTPVerificationStat) error {
	return s.createStat(ctx, stat)
}
func (s *stubRepo) GetStatsForDomain(ctx context.Context, domainID uuid.UUID, filter entities.SMTPStatsFilter, limit, offset int) ([]entities.SMTPVerificationStat, int64, error) {
	return s.getStatsForDomain(ctx, domainID, filter, limit, offset)
}
func (s *stubRepo) GetStatsForEmailAddress(ctx context.Context, emailAddressID uuid.UUID, filter entities.SMTPStatsFilter, limit, offset int) ([]entities.SMTPVerificationStat, int64, error) {
	return s.getStatsForEmailAddress(ctx, emailAddressID, filter, limit, offset)
}
func (s *stubRepo) GetOverview(ctx context.Context, filter entities.SMTPStatsFilter) (*entities.SMTPStatsOverview, error) {
	return s.getOverview(ctx, filter)
}
func (s *stubRepo) GetTimeSeriesData(ctx context.Context, filter entities.SMTPStatsFilter, granularity string) ([]entities.TimeSeriesPoint, error) {
	return s.getTimeSeriesData(ctx, filter, granularity)
}
func (s *stubRepo) GetActionDistribution(ctx context.Context, filter entities.SMTPStatsFilter) ([]entities.ActionDistribution, error) {
	return s.getActionDistribution(ctx, filter)
}
func (s *stubRepo) GetReputationDistribution(_ context.Context, _ entities.SMTPStatsFilter) ([]entities.ReputationDistribution, error) {
	return nil, nil
}
func (s *stubRepo) GetContentDistribution(_ context.Context, _ entities.SMTPStatsFilter) ([]entities.ContentDistribution, error) {
	return nil, nil
}
func (s *stubRepo) GetSPFDistribution(_ context.Context, _ entities.SMTPStatsFilter) ([]entities.SPFDistribution, error) {
	return nil, nil
}
func (s *stubRepo) GetDKIMDistribution(_ context.Context, _ entities.SMTPStatsFilter) ([]entities.DKIMDistribution, error) {
	return nil, nil
}
func (s *stubRepo) GetDMARCDistribution(_ context.Context, _ entities.SMTPStatsFilter) ([]entities.DMARCDistribution, error) {
	return nil, nil
}
func (s *stubRepo) GetTopSenderDomains(_ context.Context, _ entities.SMTPStatsFilter, _ int) ([]struct {
	Domain string `json:"domain"`
	Count  int64  `json:"count"`
}, error) {
	return nil, nil
}
func (s *stubRepo) GetTopSenderIPs(_ context.Context, _ entities.SMTPStatsFilter, _ int) ([]struct {
	IP    string `json:"ip"`
	Count int64  `json:"count"`
}, error) {
	return nil, nil
}
func (s *stubRepo) DeleteOldStats(ctx context.Context, olderThan time.Duration) (int64, error) {
	return s.deleteOldStats(ctx, olderThan)
}

func TestGetDomainStats_ClampsInvalidPaginationToDefaults(t *testing.T) {
	domainID := uuid.Must(uuid.NewV4())

	cases := []struct {
		name           string
		page, pageSize int
		wantLimit      int
		wantOffset     int
	}{
		{"both negative", -1, -1, 50, 0},
		{"page=0", 0, 20, 20, 0},
		{"pageSize=0 defaults to 50", 2, 0, 50, 50},
		{"pageSize>100 defaults to 50", 1, 200, 50, 0},
		{"valid pagination", 3, 25, 25, 50},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotLimit, gotOffset int
			repo := &stubRepo{
				getStatsForDomain: func(_ context.Context, _ uuid.UUID, _ entities.SMTPStatsFilter, limit, offset int) ([]entities.SMTPVerificationStat, int64, error) {
					gotLimit, gotOffset = limit, offset
					return nil, 0, nil
				},
			}
			uc := smtp_stats.NewUseCase(repo)
			_, _, err := uc.GetDomainStats(context.Background(), domainID, entities.SMTPStatsFilter{}, tc.page, tc.pageSize)
			require.NoError(t, err)
			assert.Equal(t, tc.wantLimit, gotLimit, "limit (pageSize) should be %d", tc.wantLimit)
			assert.Equal(t, tc.wantOffset, gotOffset, "offset should be %d", tc.wantOffset)
		})
	}
}

func TestGetEmailAddressStats_ClampsInvalidPaginationToDefaults(t *testing.T) {
	emailID := uuid.Must(uuid.NewV4())
	var gotLimit, gotOffset int
	repo := &stubRepo{
		getStatsForEmailAddress: func(_ context.Context, _ uuid.UUID, _ entities.SMTPStatsFilter, limit, offset int) ([]entities.SMTPVerificationStat, int64, error) {
			gotLimit, gotOffset = limit, offset
			return nil, 0, nil
		},
	}
	uc := smtp_stats.NewUseCase(repo)
	_, _, err := uc.GetEmailAddressStats(context.Background(), emailID, entities.SMTPStatsFilter{}, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 50, gotLimit)
	assert.Equal(t, 0, gotOffset)
}

func TestGetTimeSeriesData_DefaultsInvalidGranularityToDay(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"hour", "hour"},
		{"day", "day"},
		{"week", "week"},
		{"month", "month"},
		{"", "day"},
		{"nonsense", "day"},
		{"YEAR", "day"}, // case-sensitive
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			var got string
			repo := &stubRepo{
				getTimeSeriesData: func(_ context.Context, _ entities.SMTPStatsFilter, g string) ([]entities.TimeSeriesPoint, error) {
					got = g
					return nil, nil
				},
			}
			uc := smtp_stats.NewUseCase(repo)
			_, err := uc.GetTimeSeriesData(context.Background(), entities.SMTPStatsFilter{}, tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCleanupOldStats_RejectsRetentionBelowOneDay(t *testing.T) {
	repo := &stubRepo{
		deleteOldStats: func(_ context.Context, _ time.Duration) (int64, error) {
			t.Fatal("DeleteOldStats must not be called when retention is too short")
			return 0, nil
		},
	}
	uc := smtp_stats.NewUseCase(repo)
	for _, d := range []time.Duration{0, time.Hour, 23 * time.Hour} {
		_, err := uc.CleanupOldStats(context.Background(), d)
		require.Error(t, err, "retention %s should be rejected", d)
		assert.Contains(t, err.Error(), "24 hours")
	}
}

func TestCleanupOldStats_PassesThroughWhenValid(t *testing.T) {
	want := int64(42)
	repo := &stubRepo{
		deleteOldStats: func(_ context.Context, d time.Duration) (int64, error) {
			assert.Equal(t, 7*24*time.Hour, d)
			return want, nil
		},
	}
	uc := smtp_stats.NewUseCase(repo)
	got, err := uc.CleanupOldStats(context.Background(), 7*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGetOverview_DelegatesToRepo(t *testing.T) {
	want := &entities.SMTPStatsOverview{TotalProcessed: 99}
	repo := &stubRepo{
		getOverview: func(_ context.Context, _ entities.SMTPStatsFilter) (*entities.SMTPStatsOverview, error) {
			return want, nil
		},
	}
	uc := smtp_stats.NewUseCase(repo)
	got, err := uc.GetOverview(context.Background(), entities.SMTPStatsFilter{})
	require.NoError(t, err)
	assert.Same(t, want, got)
}

func TestGetOverview_PropagatesRepoError(t *testing.T) {
	repoErr := errors.New("pg down")
	repo := &stubRepo{
		getOverview: func(_ context.Context, _ entities.SMTPStatsFilter) (*entities.SMTPStatsOverview, error) {
			return nil, repoErr
		},
	}
	uc := smtp_stats.NewUseCase(repo)
	_, err := uc.GetOverview(context.Background(), entities.SMTPStatsFilter{})
	require.Error(t, err)
	assert.ErrorIs(t, err, repoErr)
}
