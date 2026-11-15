package v1

import "testing"

func TestRegisterWebhooks_MountsAllRoutes(t *testing.T) {
	live := routeKeys(buildFullApp(t))
	for _, route := range []string{
		"GET /v1.0/webhooks",
		"POST /v1.0/webhooks",
		"DELETE /v1.0/webhooks/{id}",
		"POST /v1.0/webhooks/{id}/rotate-secret",
		"POST /v1.0/webhooks/{id}/ping",
		"GET /v1.0/webhooks/{id}/deliveries",
		"GET /v1.0/webhooks/{id}/deliveries/{event_id}",
		"POST /v1.0/webhooks/{id}/deliveries/{event_id}/replay",
	} {
		if !live[route] {
			t.Errorf("route not mounted: %s", route)
		}
	}
}
