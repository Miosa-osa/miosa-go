package miosa

import "context"

// SnapshotsStandaloneService provides the admin fleet-wide snapshot index.
// Per-computer snapshots remain on Computer.Snapshots. Routes live under
// /api/v1/admin/snapshots/. Requires admin credentials.
type SnapshotsStandaloneService struct {
	client *Client
}

// StandaloneSnapshotData is the API representation of a fleet-wide snapshot record.
type StandaloneSnapshotData map[string]interface{}

// List returns all snapshots across the fleet, optionally filtered.
func (s *SnapshotsStandaloneService) List(ctx context.Context, params map[string]string) ([]StandaloneSnapshotData, error) {
	var wrapper struct {
		Data      []StandaloneSnapshotData `json:"data"`
		Snapshots []StandaloneSnapshotData `json:"snapshots"`
		Items     []StandaloneSnapshotData `json:"items"`
	}
	if err := s.client.getJSON(ctx, "/admin/snapshots"+buildQuery(params), &wrapper); err != nil {
		var list []StandaloneSnapshotData
		if err2 := s.client.getJSON(ctx, "/admin/snapshots"+buildQuery(params), &list); err2 == nil {
			return list, nil
		}
		return nil, err
	}
	for _, v := range [][]StandaloneSnapshotData{wrapper.Data, wrapper.Snapshots, wrapper.Items} {
		if len(v) > 0 {
			return v, nil
		}
	}
	return []StandaloneSnapshotData{}, nil
}

// Get returns a single fleet-wide snapshot by ID.
func (s *SnapshotsStandaloneService) Get(ctx context.Context, snapshotID string) (StandaloneSnapshotData, error) {
	var out StandaloneSnapshotData
	if err := s.client.getJSON(ctx, "/admin/snapshots/"+snapshotID, &out); err != nil {
		return nil, err
	}
	return out, nil
}
