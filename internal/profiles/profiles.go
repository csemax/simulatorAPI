package profiles

import (
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

var (
	ErrSimulatedTimeout      = errors.New("simulated network timeout")
	ErrSimulatedNetworkError = errors.New("simulated network error")
)

type NetworkProfile struct {
	Name         string  `json:"name"`
	BaseDelayMS int     `json:"base_delay_ms"`
	JitterMS    int     `json:"jitter_ms"`
	TimeoutRate float64 `json:"timeout_rate"`
	ErrorRate   float64 `json:"error_rate"`
}

var AvailableProfiles = map[string]NetworkProfile{
	"5g": {
		Name:         "5g",
		BaseDelayMS: 20,
		JitterMS:    10,
		TimeoutRate: 0.001,
		ErrorRate:   0.001,
	},
	"4g": {
		Name:         "4g",
		BaseDelayMS: 60,
		JitterMS:    30,
		TimeoutRate: 0.005,
		ErrorRate:   0.005,
	},
	"3g": {
		Name:         "3g",
		BaseDelayMS: 180,
		JitterMS:    100,
		TimeoutRate: 0.02,
		ErrorRate:   0.015,
	},
	"rural": {
		Name:         "rural",
		BaseDelayMS: 400,
		JitterMS:    250,
		TimeoutRate: 0.05,
		ErrorRate:   0.03,
	},
	"unstable": {
		Name:         "unstable",
		BaseDelayMS: 250,
		JitterMS:    600,
		TimeoutRate: 0.10,
		ErrorRate:   0.08,
	},
}

func Get(name string) NetworkProfile {
	normalized := strings.ToLower(strings.TrimSpace(name))

	if profile, ok := AvailableProfiles[normalized]; ok {
		return profile
	}

	return AvailableProfiles["4g"]
}

func ApplyNetworkEffect(profile NetworkProfile) (int, error) {
	delay := profile.BaseDelayMS

	if profile.JitterMS > 0 {
		delay += rand.Intn(profile.JitterMS)
	}

	time.Sleep(time.Duration(delay) * time.Millisecond)

	if rand.Float64() < profile.TimeoutRate {
		return delay, ErrSimulatedTimeout
	}

	if rand.Float64() < profile.ErrorRate {
		return delay, ErrSimulatedNetworkError
	}

	return delay, nil
}

func HandleProfiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]any{
		"default":  "4g",
		"profiles": AvailableProfiles,
	}

	_ = json.NewEncoder(w).Encode(response)
}