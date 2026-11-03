package services

// CAT-001: a failing local-media query must surface as an error, never as fabricated "Similar Movie 1" items
// (the former fallback was generateMockLocalMedia). Unit test: the repository is a double (unit-class evidence only).

import (
	"context"
	"errors"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingSearchRepo struct{ calls int }

func (f *failingSearchRepo) SearchFiles(context.Context, models.SearchFilter, models.PaginationOptions, models.SortOptions) (*models.SearchResult, error) {
	f.calls++
	return nil, errors.New("database is down")
}

func TestCAT001_FindLocalSimilarItems_QueryFailureIsAnErrorNotMockData(t *testing.T) {
	repo := &failingSearchRepo{}
	svc := NewRecommendationService(&MediaRecognitionService{}, &DuplicateDetectionService{}, repo, nil)
	req := &SimilarItemsRequest{
		MediaMetadata:    &models.MediaMetadata{ID: 1, Title: "T", MediaType: "movie", Genre: "Action"},
		MaxLocalItems:    10,
		MaxExternalItems: 5,
	}
	items, err := svc.findLocalSimilarItems(context.Background(), req)
	require.Error(t, err, "a failed query must not be papered over")
	assert.Empty(t, items, "no fabricated items")
	assert.Contains(t, err.Error(), "database is down")
	assert.Greater(t, repo.calls, 0)
}

func TestCAT001_GetSimilarItems_QueryFailurePropagates(t *testing.T) {
	svc := NewRecommendationService(&MediaRecognitionService{}, &DuplicateDetectionService{}, &failingSearchRepo{}, nil)
	resp, err := svc.GetSimilarItems(context.Background(), &SimilarItemsRequest{
		MediaMetadata: &models.MediaMetadata{ID: 1, MediaType: "movie", Genre: "Action"}, MaxLocalItems: 10,
	})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestCAT001_FindLocalSimilarItems_NilMetadataIsAnErrorNotMockData(t *testing.T) {
	svc := NewRecommendationService(&MediaRecognitionService{}, &DuplicateDetectionService{}, &failingSearchRepo{}, nil)
	items, err := svc.findLocalSimilarItems(context.Background(), &SimilarItemsRequest{MaxLocalItems: 10})
	require.Error(t, err)
	assert.Empty(t, items)
}
