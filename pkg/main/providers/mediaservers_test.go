package providers

import "testing"

func TestMediaServerLimiterDefaults(t *testing.T) {
	tests := []struct {
		seconds, wantSeconds uint8
		calls, wantCalls     int
	}{
		{0, 1, 0, 10},
		{5, 5, 0, 10},
		{0, 1, 3, 3},
		{2, 2, 20, 20},
	}

	for _, tt := range tests {
		s, c := mediaServerLimiterDefaults(tt.seconds, tt.calls)
		if s != tt.wantSeconds || c != tt.wantCalls {
			t.Errorf("mediaServerLimiterDefaults(%d, %d) = %d, %d; want %d, %d",
				tt.seconds, tt.calls, s, c, tt.wantSeconds, tt.wantCalls)
		}
	}
}
