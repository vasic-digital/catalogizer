package services

import (
	"net/http"
	"testing"

	"catalogizer/internal/netguard"
	"catalogizer/internal/netguard/netguardtest"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// AZ2-1: every fixed-host provider client must refuse a redirect into 0.0.0.0 (an address class netguard never
// admits). Before the fix the plain http.Client followed it and the sink was hit (RED log:
// docs/qa/az21-20261117/red_services_baseline.log, this same test at the baseline commit).
func az21Members() map[string]func() *http.Client {
	lg := zap.NewNop()
	return map[string]func() *http.Client{
		"igdb.http":         func() *http.Client { return NewIGDBResolver(1).http },
		"caa.http":          func() *http.Client { return NewCoverArtArchiveResolver(1).http },
		"recognition.game":  func() *http.Client { return NewGameSoftwareRecognitionProvider(lg).httpClient },
		"recognition.music": func() *http.Client { return NewMusicRecognitionProvider(lg).httpClient },
		"recognition.movie": func() *http.Client { return NewMovieRecognitionProvider(lg).httpClient },
		"recognition.book":  func() *http.Client { return NewBookRecognitionProvider(lg).httpClient },
		"lyrics":            func() *http.Client { return NewLyricsService(nil, lg).httpClient },
		"translation":       func() *http.Client { return NewTranslationService(lg).httpClient },
		"llm_image.client":  func() *http.Client { return NewLLMImageSearchResolver(1).client },
		"recommendation":    func() *http.Client { return NewRecommendationService(nil, nil, nil, nil).httpClient },
	}
}

func TestAZ21_ProviderClientsRefuseRedirectToBlockedClass(t *testing.T) {
	AllowPrivateSSRFForTests(t)
	for name, mk := range az21Members() {
		t.Run(name, func(t *testing.T) {
			top := netguardtest.New(t)
			c := mk()
			// control needle: the instrument sees the front host and the sink counter works
			r, err := c.Get(top.FrontURL + "/ok")
			require.NoError(t, err)
			r.Body.Close()
			r, err = c.Get(top.SinkURL + "/secret")
			require.NoError(t, err)
			r.Body.Close()
			base := top.SinkHits()
			require.Equal(t, 1, base, "control needle: sink counter")
			resp, err := c.Get(top.FrontURL + "/to-unspecified")
			if resp != nil {
				resp.Body.Close()
			}
			if err != nil {
				require.ErrorIs(t, err, netguard.ErrBlocked, "the refusal must come from the class check")
			}
			require.Equal(t, base, top.SinkHits(), "client followed a redirect to 0.0.0.0 and reached the sink")
			// the scheme class is refused too
			_, err = c.Get(top.FrontURL + "/to-gopher")
			require.Error(t, err)
		})
	}
}
