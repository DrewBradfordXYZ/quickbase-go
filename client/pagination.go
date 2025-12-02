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

			// Check for more pages
			if metadata.NextPageToken != nil && *metadata.NextPageToken != "" {
				// Token-based pagination
				nextToken = *metadata.NextPageToken
			} else if metadata.TotalRecords != nil && metadata.Skip != nil && metadata.NumRecords != nil {
				// Skip-based pagination
				currentEnd := *metadata.Skip + *metadata.NumRecords
				if currentEnd >= *metadata.TotalRecords {
					return // No more pages
				}
				skip = currentEnd
				nextToken = ""
			} else {
				// No pagination info or single page
				return
			}
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
