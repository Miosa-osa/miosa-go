package miosa

import "context"

// AnalyticsService provides access to admin-scoped platform analytics.
type AnalyticsService struct {
	client *Client
}

// AnalyticsOverviewInput holds optional query filters for GET /analytics/overview.
type AnalyticsOverviewInput struct {
	Period    string
	Region    string
	Workspace string
}

// AnalyticsTimeseriesInput holds query parameters for GET /analytics/timeseries.
type AnalyticsTimeseriesInput struct {
	Metric string
	Period string
	Region string
}

// Overview returns the platform analytics overview with optional filters.
func (s *AnalyticsService) Overview(ctx context.Context, input AnalyticsOverviewInput) (map[string]interface{}, error) {
	params := map[string]string{}
	if input.Period != "" {
		params["period"] = input.Period
	}
	if input.Region != "" {
		params["region"] = input.Region
	}
	if input.Workspace != "" {
		params["workspace"] = input.Workspace
	}
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/analytics/overview"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Timeseries returns a timeseries for a metric over a period.
func (s *AnalyticsService) Timeseries(ctx context.Context, input AnalyticsTimeseriesInput) (map[string]interface{}, error) {
	params := map[string]string{}
	if input.Metric != "" {
		params["metric"] = input.Metric
	}
	if input.Period != "" {
		params["period"] = input.Period
	}
	if input.Region != "" {
		params["region"] = input.Region
	}
	var out map[string]interface{}
	if err := s.client.getJSON(ctx, "/analytics/timeseries"+buildQuery(params), &out); err != nil {
		return nil, err
	}
	return out, nil
}
