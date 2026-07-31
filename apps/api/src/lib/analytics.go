package lib

import (
	"context"
	"os"
	"strings"

	mp "github.com/mixpanel/mixpanel-go"
)

var mpClient *mp.ApiClient

func InitAnalytics() {
	// ITAR/CUI boundary: never egress analytics from the GovCloud partition,
	// even if a token is configured. Mixpanel ingests outside the boundary.
	if strings.HasPrefix(strings.ToLower(os.Getenv("AWS_REGION")), "us-gov-") {
		return
	}
	token := os.Getenv("MIXPANEL_TOKEN")
	if token != "" {
		mpClient = mp.NewApiClient(token)
	}
}

func Track(event string, distinctID string, props map[string]any) {
	if mpClient == nil {
		return
	}
	mpClient.Track(context.Background(), []*mp.Event{
		mpClient.NewEvent(event, distinctID, props),
	})
}
