package services

import (
	"context"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
)

// CAT-001: IGDB / Steam "similar games" had no client and returned invented
// titles, prices and scores. They must report not-implemented instead.
func TestRecommendationService_SimilarGamesProvidersDoNotFabricate(t *testing.T) {
	rs := &RecommendationService{}
	md := &models.MediaMetadata{Genre: "rpg"}

	igdb, err := rs.getIGDBSimilarGames(context.Background(), md)
	assert.ErrorIs(t, err, ErrRecommendationProviderNotImplemented)
	assert.Empty(t, igdb)

	steam, err := rs.getSteamSimilarGames(context.Background(), md)
	assert.ErrorIs(t, err, ErrRecommendationProviderNotImplemented)
	assert.Empty(t, steam)

	items, err := rs.findSimilarGames(context.Background(), md)
	assert.NoError(t, err)
	assert.Empty(t, items)
}
