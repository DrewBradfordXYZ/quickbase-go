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

func TestDetectPaginationType(t *testing.T) {
	tests := []struct {
		name     string
		metadata PaginationMetadata
		expected PaginationType
	}{
		{
			name: "skip-based pagination",
			metadata: PaginationMetadata{
				TotalRecords: intPtr(100),
				Skip:         intPtr(0),
			},
			expected: PaginationTypeSkip,
		},
		{
			name: "token-based pagination with NextPageToken",
			metadata: PaginationMetadata{
				NextPageToken: strPtr("token123"),
			},
			expected: PaginationTypeToken,
		},
		{
			name: "token-based pagination with NextToken",
			metadata: PaginationMetadata{
				NextToken: strPtr("token456"),
			},
			expected: PaginationTypeToken,
		},
		{
			name:     "no pagination",
			metadata: PaginationMetadata{},
			expected: PaginationTypeNone,
		},
		{
			name: "only NumRecords (no pagination)",
			metadata: PaginationMetadata{
				NumRecords: intPtr(10),
			},
			expected: PaginationTypeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectPaginationType(tt.metadata)
			if result != tt.expected {
				t.Errorf("DetectPaginationType() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHasMorePages(t *testing.T) {
	tests := []struct {
		name         string
		metadata     PaginationMetadata
		ptype        PaginationType
		currentCount int
		expected     bool
	}{
		{
			name: "skip-based with more pages",
			metadata: PaginationMetadata{
				TotalRecords: intPtr(100),
			},
			ptype:        PaginationTypeSkip,
			currentCount: 50,
			expected:     true,
		},
		{
			name: "skip-based no more pages",
			metadata: PaginationMetadata{
				TotalRecords: intPtr(100),
			},
			ptype:        PaginationTypeSkip,
			currentCount: 100,
			expected:     false,
		},
		{
			name: "skip-based exceeded total",
			metadata: PaginationMetadata{
				TotalRecords: intPtr(100),
			},
			ptype:        PaginationTypeSkip,
			currentCount: 150,
			expected:     false,
		},
		{
			name: "token-based with NextPageToken",
			metadata: PaginationMetadata{
				NextPageToken: strPtr("next-token"),
			},
			ptype:        PaginationTypeToken,
			currentCount: 50,
			expected:     true,
		},
		{
			name: "token-based with empty NextPageToken",
			metadata: PaginationMetadata{
				NextPageToken: strPtr(""),
			},
			ptype:        PaginationTypeToken,
			currentCount: 50,
			expected:     false,
		},
		{
			name: "token-based with NextToken",
			metadata: PaginationMetadata{
				NextToken: strPtr("next-token"),
			},
			ptype:        PaginationTypeToken,
			currentCount: 50,
			expected:     true,
		},
		{
			name:         "no pagination type",
			metadata:     PaginationMetadata{},
			ptype:        PaginationTypeNone,
			currentCount: 50,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasMorePages(tt.metadata, tt.ptype, tt.currentCount)
			if result != tt.expected {
				t.Errorf("HasMorePages() = %v, want %v", result, tt.expected)
			}
		})
	}
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
					data: []mockRecord{{ID: 5}},
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

func TestPaginatedRequest(t *testing.T) {
	t.Run("Execute returns single page when autoPaginate is false", func(t *testing.T) {
		ctx := context.Background()
		callCount := 0

		fetcher := func(ctx context.Context, skip int, nextToken string) (*mockResponse, error) {
			callCount++
			return &mockResponse{
				data: []mockRecord{{ID: 1}, {ID: 2}},
				metadata: PaginationMetadata{
					TotalRecords: intPtr(10),
					NumRecords:   intPtr(2),
					Skip:         intPtr(0),
				},
			}, nil
		}

		req := NewPaginatedRequest(ctx, fetcher, false)
		resp, err := req.Execute()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(resp.GetData()) != 2 {
			t.Errorf("got %d records, want 2", len(resp.GetData()))
		}
		if callCount != 1 {
			t.Errorf("fetcher called %d times, want 1", callCount)
		}
	})

	t.Run("Execute returns all pages when autoPaginate is true", func(t *testing.T) {
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

		req := NewPaginatedRequest(ctx, fetcher, true)
		_, err := req.Execute()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if callCount != 2 {
			t.Errorf("fetcher called %d times, want 2", callCount)
		}
	})

	t.Run("NoPaginate returns single page", func(t *testing.T) {
		ctx := context.Background()
		callCount := 0

		fetcher := func(ctx context.Context, skip int, nextToken string) (*mockResponse, error) {
			callCount++
			return &mockResponse{
				data: []mockRecord{{ID: 1}},
				metadata: PaginationMetadata{
					TotalRecords: intPtr(10),
					NumRecords:   intPtr(1),
					Skip:         intPtr(0),
				},
			}, nil
		}

		req := NewPaginatedRequest(ctx, fetcher, true) // autoPaginate is true
		resp, err := req.NoPaginate()                  // but we explicitly call NoPaginate
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(resp.GetData()) != 1 {
			t.Errorf("got %d records, want 1", len(resp.GetData()))
		}
		if callCount != 1 {
			t.Errorf("fetcher called %d times, want 1", callCount)
		}
	})

	t.Run("All returns all pages", func(t *testing.T) {
		ctx := context.Background()
		callCount := 0

		fetcher := func(ctx context.Context, skip int, nextToken string) (*mockResponse, error) {
			callCount++
			switch callCount {
			case 1:
				return &mockResponse{
					data: []mockRecord{{ID: 1}},
					metadata: PaginationMetadata{
						TotalRecords: intPtr(2),
						NumRecords:   intPtr(1),
						Skip:         intPtr(0),
					},
				}, nil
			case 2:
				return &mockResponse{
					data: []mockRecord{{ID: 2}},
					metadata: PaginationMetadata{
						TotalRecords: intPtr(2),
						NumRecords:   intPtr(1),
						Skip:         intPtr(1),
					},
				}, nil
			default:
				t.Fatal("unexpected call to fetcher")
				return nil, nil
			}
		}

		req := NewPaginatedRequest(ctx, fetcher, false) // autoPaginate is false
		_, err := req.All()                             // but we explicitly call All
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if callCount != 2 {
			t.Errorf("fetcher called %d times, want 2", callCount)
		}
	})

	t.Run("Paginate respects limit", func(t *testing.T) {
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

		req := NewPaginatedRequest(ctx, fetcher, false)
		_, err := req.Paginate(PaginationOptions{Limit: 3})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Should stop after getting 3 records (needs 2 pages with 2 records each)
		if callCount != 2 {
			t.Errorf("fetcher called %d times, want 2", callCount)
		}
	})
}
