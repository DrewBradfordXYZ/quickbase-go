package client

import (
	"context"
	"testing"
)

// mockResponse implements PaginatedResponse for testing
type mockResponse struct {
	data     []mockRecord
	metadata PaginationMetadata
}

type mockRecord struct {
	ID   int
	Name string
}

func (r *mockResponse) GetData() []mockRecord {
	return r.data
}

func (r *mockResponse) GetMetadata() PaginationMetadata {
	return r.metadata
}

func intPtr(i int) *int {
	return &i
}

func strPtr(s string) *string {
	return &s
}

func TestPaginate(t *testing.T) {
	t.Run("iterates through skip-based pages", func(t *testing.T) {
		ctx := context.Background()
		callCount := 0

		fetcher := func(ctx context.Context, skip int, nextToken string) (*mockResponse, error) {
			callCount++
			switch callCount {
			case 1:
				return &mockResponse{
					data: []mockRecord{{ID: 1}, {ID: 2}, {ID: 3}},
					metadata: PaginationMetadata{
						TotalRecords: intPtr(7),
						NumRecords:   intPtr(3),
						Skip:         intPtr(0),
					},
				}, nil
			case 2:
				return &mockResponse{
					data: []mockRecord{{ID: 4}, {ID: 5}, {ID: 6}},
					metadata: PaginationMetadata{
						TotalRecords: intPtr(7),
						NumRecords:   intPtr(3),
						Skip:         intPtr(3),
					},
				}, nil
			case 3:
				return &mockResponse{
					data: []mockRecord{{ID: 7}},
					metadata: PaginationMetadata{
						TotalRecords: intPtr(7),
						NumRecords:   intPtr(1),
						Skip:         intPtr(6),
					},
				}, nil
			default:
				t.Fatal("unexpected call to fetcher")
				return nil, nil
			}
		}

		var records []mockRecord
		for record, err := range Paginate(ctx, fetcher) {
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			records = append(records, record)
		}

		if len(records) != 7 {
			t.Errorf("got %d records, want 7", len(records))
		}
		if callCount != 3 {
			t.Errorf("fetcher called %d times, want 3", callCount)
		}
	})

	t.Run("iterates through token-based pages", func(t *testing.T) {
		ctx := context.Background()
		callCount := 0

		fetcher := func(ctx context.Context, skip int, nextToken string) (*mockResponse, error) {
			callCount++
			switch callCount {
			case 1:
				return &mockResponse{
					data: []mockRecord{{ID: 1}, {ID: 2}},
					metadata: PaginationMetadata{
						NextPageToken: strPtr("page2"),
					},
				}, nil
			case 2:
				return &mockResponse{
					data: []mockRecord{{ID: 3}, {ID: 4}},
					metadata: PaginationMetadata{
						NextPageToken: strPtr("page3"),
					},
				}, nil
			case 3:
				return &mockResponse{
					data:     []mockRecord{{ID: 5}},
					metadata: PaginationMetadata{
						// No NextPageToken - last page
					},
				}, nil
			default:
				t.Fatal("unexpected call to fetcher")
				return nil, nil
			}
		}

		var records []mockRecord
		for record, err := range Paginate(ctx, fetcher) {
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			records = append(records, record)
		}

		if len(records) != 5 {
			t.Errorf("got %d records, want 5", len(records))
		}
		if callCount != 3 {
			t.Errorf("fetcher called %d times, want 3", callCount)
		}
	})

	t.Run("handles single page response", func(t *testing.T) {
		ctx := context.Background()
		callCount := 0

		fetcher := func(ctx context.Context, skip int, nextToken string) (*mockResponse, error) {
			callCount++
			return &mockResponse{
				data: []mockRecord{{ID: 1}, {ID: 2}},
				metadata: PaginationMetadata{
					TotalRecords: intPtr(2),
					NumRecords:   intPtr(2),
					Skip:         intPtr(0),
				},
			}, nil
		}

		var records []mockRecord
		for record, err := range Paginate(ctx, fetcher) {
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			records = append(records, record)
		}

		if len(records) != 2 {
			t.Errorf("got %d records, want 2", len(records))
		}
		if callCount != 1 {
			t.Errorf("fetcher called %d times, want 1", callCount)
		}
	})
}

func TestCollectAll(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	fetcher := func(ctx context.Context, skip int, nextToken string) (*mockResponse, error) {
		callCount++
		switch callCount {
		case 1:
			return &mockResponse{
				data: []mockRecord{{ID: 1}, {ID: 2}},
				metadata: PaginationMetadata{
					TotalRecords: intPtr(4),
					NumRecords:   intPtr(2),
					Skip:         intPtr(0),
				},
			}, nil
		case 2:
			return &mockResponse{
				data: []mockRecord{{ID: 3}, {ID: 4}},
				metadata: PaginationMetadata{
					TotalRecords: intPtr(4),
					NumRecords:   intPtr(2),
					Skip:         intPtr(2),
				},
			}, nil
		default:
			t.Fatal("unexpected call to fetcher")
			return nil, nil
		}
	}

	records, err := CollectAll(ctx, fetcher)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(records) != 4 {
		t.Errorf("got %d records, want 4", len(records))
	}
}

func TestCollectN(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	fetcher := func(ctx context.Context, skip int, nextToken string) (*mockResponse, error) {
		callCount++
		return &mockResponse{
			data: []mockRecord{{ID: callCount*2 - 1}, {ID: callCount * 2}},
			metadata: PaginationMetadata{
				TotalRecords: intPtr(100),
				NumRecords:   intPtr(2),
				Skip:         intPtr((callCount - 1) * 2),
			},
		}, nil
	}

	records, err := CollectN(ctx, fetcher, 5)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(records) != 5 {
		t.Errorf("got %d records, want 5", len(records))
	}
	if callCount != 3 {
		t.Errorf("fetcher called %d times, want 3 (to get 5 records with 2 per page)", callCount)
	}
}
