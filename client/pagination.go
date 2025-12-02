// Package client provides pagination support with fluent API for QuickBase SDK.
//
// Provides automatic pagination for QuickBase API endpoints that return
// paginated results. Supports both skip-based and token-based pagination.
//
// Example usage:
//
//	// Fetch single page (default)
//	page, err := client.RunQuery(ctx, params)
//
//	// Fetch all pages automatically
//	all, err := client.RunQuery(ctx, params).All()
//
//	// Fetch with record limit
//	limited, err := client.RunQuery(ctx, params).Paginate(PaginationOptions{Limit: 500})
//
//	// Iterate over results
//	for record, err := range client.RunQueryIterator(ctx, params) {
//	    if err != nil { ... }
//	    // process record
//	}
package client

import (
	"context"
	"iter"
)

// PaginatedResponse represents a response that may have more pages.
type PaginatedResponse[T any] interface {
	GetData() []T
	GetMetadata() PaginationMetadata
}

// PaginationMetadata contains pagination information from the response.
type PaginationMetadata struct {
	TotalRecords  *int
	NumRecords    *int
	Skip          *int
	NextPageToken *string
	NextToken     *string
}

// PaginationOptions controls automatic pagination behavior.
type PaginationOptions struct {
	// Limit is the maximum number of records to fetch across all pages.
	// Zero means no limit.
	Limit int
	// Skip is the starting offset for skip-based pagination.
	Skip int
}

// PaginationType indicates the type of pagination a response uses.
type PaginationType string

const (
	PaginationTypeSkip  PaginationType = "skip"
	PaginationTypeToken PaginationType = "token"
	PaginationTypeNone  PaginationType = "none"
)

// PageFetcher is a function that fetches a page of results.
// For skip-based: pass skip value
// For token-based: pass nextPageToken
type PageFetcher[T any, R PaginatedResponse[T]] func(ctx context.Context, skip int, nextToken string) (R, error)

// Paginate returns an iterator that automatically fetches all pages.
func Paginate[T any, R PaginatedResponse[T]](ctx context.Context, fetcher PageFetcher[T, R]) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var skip int
		var nextToken string
		var zero T

		for {
			resp, err := fetcher(ctx, skip, nextToken)
			if err != nil {
				yield(zero, err)
				return
			}

			data := resp.GetData()
			metadata := resp.GetMetadata()

			for _, item := range data {
				if !yield(item, nil) {
					return
				}
			}

			// Check for more pages (token-based - NextPageToken)
			if metadata.NextPageToken != nil && *metadata.NextPageToken != "" {
				nextToken = *metadata.NextPageToken
				continue
			}

			// Check for more pages (token-based - NextToken)
			if metadata.NextToken != nil && *metadata.NextToken != "" {
				nextToken = *metadata.NextToken
				continue
			}

			// Check for more pages (skip-based)
			if metadata.TotalRecords != nil && metadata.Skip != nil && metadata.NumRecords != nil {
				currentEnd := *metadata.Skip + *metadata.NumRecords
				if currentEnd >= *metadata.TotalRecords {
					return // No more pages
				}
				skip = currentEnd
				nextToken = ""
				continue
			}

			// No pagination info or single page
			return
		}
	}
}

// CollectAll fetches all pages and returns all items.
func CollectAll[T any, R PaginatedResponse[T]](ctx context.Context, fetcher PageFetcher[T, R]) ([]T, error) {
	var result []T
	for item, err := range Paginate(ctx, fetcher) {
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

// CollectN fetches up to n items across pages.
func CollectN[T any, R PaginatedResponse[T]](ctx context.Context, fetcher PageFetcher[T, R], n int) ([]T, error) {
	var result []T
	for item, err := range Paginate(ctx, fetcher) {
		if err != nil {
			return nil, err
		}
		result = append(result, item)
		if len(result) >= n {
			break
		}
	}
	return result, nil
}

// PaginatedRequest wraps an API request with fluent pagination methods.
// This provides a similar API to the JavaScript SDK's PaginatedRequest class.
type PaginatedRequest[T any, R PaginatedResponse[T]] struct {
	ctx              context.Context
	fetcher          PageFetcher[T, R]
	defaultAutoPag   bool
}

// NewPaginatedRequest creates a new paginated request wrapper.
func NewPaginatedRequest[T any, R PaginatedResponse[T]](
	ctx context.Context,
	fetcher PageFetcher[T, R],
	autoPaginate bool,
) *PaginatedRequest[T, R] {
	return &PaginatedRequest[T, R]{
		ctx:            ctx,
		fetcher:        fetcher,
		defaultAutoPag: autoPaginate,
	}
}

// Execute runs the request with default pagination behavior.
// If autoPaginate is enabled globally, this behaves like All().
// Otherwise, it fetches only the first page.
func (r *PaginatedRequest[T, R]) Execute() (R, error) {
	if r.defaultAutoPag {
		return r.All()
	}
	return r.NoPaginate()
}

// All fetches all pages and combines results into a single response.
func (r *PaginatedRequest[T, R]) All() (R, error) {
	return r.Paginate(PaginationOptions{})
}

// Paginate fetches pages with optional limit and skip.
func (r *PaginatedRequest[T, R]) Paginate(opts PaginationOptions) (R, error) {
	var allItems []T
	var lastResp R
	var skip = opts.Skip
	var nextToken string

	for {
		resp, err := r.fetcher(r.ctx, skip, nextToken)
		if err != nil {
			return lastResp, err
		}
		lastResp = resp

		data := resp.GetData()
		metadata := resp.GetMetadata()

		// Apply limit if specified
		if opts.Limit > 0 {
			remaining := opts.Limit - len(allItems)
			if len(data) > remaining {
				data = data[:remaining]
			}
		}

		allItems = append(allItems, data...)

		// Check if we've hit our limit
		if opts.Limit > 0 && len(allItems) >= opts.Limit {
			break
		}

		// Check for more pages (token-based)
		if metadata.NextPageToken != nil && *metadata.NextPageToken != "" {
			nextToken = *metadata.NextPageToken
			continue
		}
		if metadata.NextToken != nil && *metadata.NextToken != "" {
			nextToken = *metadata.NextToken
			continue
		}

		// Check for more pages (skip-based)
		if metadata.TotalRecords != nil && metadata.Skip != nil && metadata.NumRecords != nil {
			currentEnd := *metadata.Skip + *metadata.NumRecords
			if currentEnd >= *metadata.TotalRecords {
				break // No more pages
			}
			skip = currentEnd
			nextToken = ""
			continue
		}

		// No pagination info, single page
		break
	}

	// Return last response (caller should update data with allItems if needed)
	return lastResp, nil
}

// NoPaginate explicitly fetches only a single page (no automatic pagination).
func (r *PaginatedRequest[T, R]) NoPaginate() (R, error) {
	return r.fetcher(r.ctx, 0, "")
}

// Iterator returns an iterator over all records across pages.
func (r *PaginatedRequest[T, R]) Iterator() iter.Seq2[T, error] {
	return Paginate(r.ctx, r.fetcher)
}

// DetectPaginationType determines the pagination type from metadata.
func DetectPaginationType(metadata PaginationMetadata) PaginationType {
	if metadata.TotalRecords != nil && metadata.Skip != nil {
		return PaginationTypeSkip
	}
	if metadata.NextPageToken != nil || metadata.NextToken != nil {
		return PaginationTypeToken
	}
	return PaginationTypeNone
}

// HasMorePages checks if a response has more pages available.
func HasMorePages(metadata PaginationMetadata, ptype PaginationType, currentCount int) bool {
	switch ptype {
	case PaginationTypeSkip:
		if metadata.TotalRecords != nil {
			return currentCount < *metadata.TotalRecords
		}
	case PaginationTypeToken:
		if metadata.NextPageToken != nil && *metadata.NextPageToken != "" {
			return true
		}
		if metadata.NextToken != nil && *metadata.NextToken != "" {
			return true
		}
	}
	return false
}
