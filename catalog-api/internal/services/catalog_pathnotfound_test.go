package services

import (
	"errors"

)

// ListPath on a path that is not cataloged must return ErrPathNotFound so callers (the download handler) can answer 404, not 500.
func (suite *CatalogServiceTestSuite) TestListPathUnknownPathIsErrPathNotFound() {
	_, err := suite.service.ListDirectory("no-such-dir-xyz")
	suite.Require().Error(err)
	suite.True(errors.Is(err, ErrPathNotFound), "got %v", err)
}

