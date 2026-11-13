// Pagination: automatic pagination for QuickBase API endpoints that return
// paginated results. Supports both skip-based and token-based pagination.
//
// Example usage:
//
//	// Fetch single page (default)
//	page, err := client.RunQuery(ctx, params)
//
//	// Fetch all pages
//	all, err := client.RunQueryAll(ctx, body)
//
//	// Fetch up to 500 records
//	limited, err := client.RunQueryN(ctx, body, 500)
//
//	// Iterate over results, a page at a time
//	for record, err := range client.RunQueryIterator(ctx, body) {
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
