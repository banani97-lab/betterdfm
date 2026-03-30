package lib

import (
	"context"
	"os"

	mp "github.com/mixpanel/mixpanel-go"
)

var mpClient *mp.ApiClient

func InitAnalytics() {
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
