package deploy

import "testing"

func TestQualifyProcessName(t *testing.T) {
	cases := []struct {
		project, app, want string
	}{
		{"shop", "web", "shop-web"},
		{"Shop App", "Web API", "shop-app-web-api"},
		{"shop", "shop", "shop"},
		{"shop", "", "shop"},
		{"", "web", "web"},
		{"", "", "app"},
		{"shop", "shop-web", "shop-web"},
		{"College Connect", "api", "college-connect-api"},
	}
	for _, tc := range cases {
		got := QualifyProcessName(tc.project, tc.app)
		if got != tc.want {
			t.Errorf("QualifyProcessName(%q, %q) = %q, want %q", tc.project, tc.app, got, tc.want)
		}
	}
}
