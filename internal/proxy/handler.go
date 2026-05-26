package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dante-simulator-api/internal/config"
	"dante-simulator-api/internal/profiles"
)

type Handler struct {
	cfg    config.Config
	client *http.Client
}

type AccessLog struct {
	RequestID        string `json:"request_id"`
	Method           string `json:"method"`
	Path             string `json:"path"`
	Target           string `json:"target"`
	TargetURL        string `json:"target_url"`
	NetworkProfile   string `json:"network_profile"`
	StatusCode        int    `json:"status_code"`
	DurationMS       int64  `json:"duration_ms"`
	SimulatedDelayMS int    `json:"simulated_delay_ms"`
	Error            string `json:"error,omitempty"`
	Timestamp        string `json:"timestamp"`
}

func NewHandler(cfg config.Config, client *http.Client) *Handler {
	return &Handler{
		cfg:    cfg,
		client: client,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	if !isAllowedProxyPath(r.URL.Path) {
		writeJSONError(w, http.StatusForbidden, "proxy_path_not_allowed", "this path is not allowed by simulator")
		return
	}

	useDante := parseUseDante(r)
	networkProfileName := parseNetworkProfile(r)
	networkProfile := profiles.Get(networkProfileName)

	target := "legacy"
	targetBaseURL := h.cfg.LegacyBaseURL

	if useDante {
		target = "dante"
		targetBaseURL = h.cfg.DanteBaseURL
	}

	targetURL, err := buildTargetURL(targetBaseURL, r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_target_url", err.Error())
		return
	}

	totalSimulatedDelay := 0

	delay, err := profiles.ApplyNetworkEffect(networkProfile)
	totalSimulatedDelay += delay
	if err != nil {
		duration := time.Since(start)
		status := simulatedStatusCode(err)

		writeSimulatedError(w, err, target, networkProfile.Name, duration, totalSimulatedDelay)
		logAccess(r, target, targetURL, networkProfile.Name, status, duration, totalSimulatedDelay, err)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_body", "failed to read request body")
		return
	}
	defer r.Body.Close()

	req, err := http.NewRequestWithContext(
		r.Context(),
		r.Method,
		targetURL,
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "proxy_request_failed", err.Error())
		return
	}

	copyRequestHeaders(req.Header, r.Header)

	req.Header.Set("X-Simulator-Target", target)
	req.Header.Set("X-Simulator-Network-Profile", networkProfile.Name)
	req.Header.Set("X-Simulator-Use-Dante", strconv.FormatBool(useDante))

	resp, err := h.client.Do(req)
	if err != nil {
		duration := time.Since(start)

		writeProxyError(w, err, target, networkProfile.Name, duration, totalSimulatedDelay)
		logAccess(r, target, targetURL, networkProfile.Name, http.StatusBadGateway, duration, totalSimulatedDelay, err)
		return
	}
	defer resp.Body.Close()

	delay, err = profiles.ApplyNetworkEffect(networkProfile)
	totalSimulatedDelay += delay
	if err != nil {
		duration := time.Since(start)
		status := simulatedStatusCode(err)

		writeSimulatedError(w, err, target, networkProfile.Name, duration, totalSimulatedDelay)
		logAccess(r, target, targetURL, networkProfile.Name, status, duration, totalSimulatedDelay, err)
		return
	}

	copyResponseHeaders(w.Header(), resp.Header)

	duration := time.Since(start)

	w.Header().Set("X-Simulator-Target", target)
	w.Header().Set("X-Simulator-Network-Profile", networkProfile.Name)
	w.Header().Set("X-Simulator-Use-Dante", strconv.FormatBool(useDante))
	w.Header().Set("X-Simulator-Duration-MS", strconv.FormatInt(duration.Milliseconds(), 10))
	w.Header().Set("X-Simulator-Simulated-Delay-MS", strconv.Itoa(totalSimulatedDelay))

	w.WriteHeader(resp.StatusCode)

	_, copyErr := io.Copy(w, resp.Body)
	if copyErr != nil {
		logAccess(r, target, targetURL, networkProfile.Name, resp.StatusCode, duration, totalSimulatedDelay, copyErr)
		return
	}

	logAccess(r, target, targetURL, networkProfile.Name, resp.StatusCode, duration, totalSimulatedDelay, nil)
}

func parseUseDante(r *http.Request) bool {
	value := r.Header.Get("X-Use-Dante")
	if value == "" {
		value = r.URL.Query().Get("useDante")
	}

	value = strings.ToLower(strings.TrimSpace(value))

	return value == "true" || value == "1" || value == "yes" || value == "on"
}

func parseNetworkProfile(r *http.Request) string {
	value := r.Header.Get("X-Network-Profile")
	if value == "" {
		value = r.URL.Query().Get("networkProfile")
	}

	value = strings.ToLower(strings.TrimSpace(value))

	if value == "" {
		return "4g"
	}

	return value
}

func buildTargetURL(baseURL string, r *http.Request) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	path := strings.TrimPrefix(r.URL.Path, "/proxy")
	if path == "" {
		path = "/"
	}

	basePath := strings.TrimRight(base.Path, "/")
	proxyPath := "/" + strings.TrimLeft(path, "/")

	base.Path = basePath + proxyPath

	query := r.URL.Query()
	query.Del("useDante")
	query.Del("networkProfile")

	base.RawQuery = query.Encode()

	return base.String(), nil
}

func isAllowedProxyPath(path string) bool {
	allowedPrefixes := []string{
		"/proxy/health",
		"/proxy/ready",
		"/proxy/v1/accounts/",
		"/proxy/v1/transactions/",
		"/proxy/v1/merchants/",
	}

	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

func copyRequestHeaders(dst, src http.Header) {
	for key, values := range src {
		if shouldSkipRequestHeader(key) {
			continue
		}

		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func copyResponseHeaders(dst, src http.Header) {
	for key, values := range src {
		if shouldSkipResponseHeader(key) {
			continue
		}

		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func shouldSkipRequestHeader(key string) bool {
	switch strings.ToLower(key) {
	case "host",
		"connection",
		"content-length",
		"x-use-dante",
		"x-network-profile":
		return true
	default:
		return false
	}
}

func shouldSkipResponseHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection",
		"content-length",
		"transfer-encoding":
		return true
	default:
		return false
	}
}

func simulatedStatusCode(err error) int {
	if errors.Is(err, profiles.ErrSimulatedTimeout) {
		return http.StatusGatewayTimeout
	}

	return http.StatusServiceUnavailable
}

func writeSimulatedError(
	w http.ResponseWriter,
	err error,
	target string,
	profile string,
	duration time.Duration,
	simulatedDelayMS int,
) {
	statusCode := simulatedStatusCode(err)
	errorCode := "simulated_network_error"

	if errors.Is(err, profiles.ErrSimulatedTimeout) {
		errorCode = "simulated_timeout"
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Simulator-Target", target)
	w.Header().Set("X-Simulator-Network-Profile", profile)
	w.Header().Set("X-Simulator-Duration-MS", strconv.FormatInt(duration.Milliseconds(), 10))
	w.Header().Set("X-Simulator-Simulated-Delay-MS", strconv.Itoa(simulatedDelayMS))
	w.WriteHeader(statusCode)

	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":                         errorCode,
		"message":                       err.Error(),
		"target":                        target,
		"network_profile":               profile,
		"simulator_duration_ms":         duration.Milliseconds(),
		"simulator_simulated_delay_ms":  simulatedDelayMS,
	})
}

func writeProxyError(
	w http.ResponseWriter,
	err error,
	target string,
	profile string,
	duration time.Duration,
	simulatedDelayMS int,
) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Simulator-Target", target)
	w.Header().Set("X-Simulator-Network-Profile", profile)
	w.Header().Set("X-Simulator-Duration-MS", strconv.FormatInt(duration.Milliseconds(), 10))
	w.Header().Set("X-Simulator-Simulated-Delay-MS", strconv.Itoa(simulatedDelayMS))
	w.WriteHeader(http.StatusBadGateway)

	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":                         "proxy_error",
		"message":                       err.Error(),
		"target":                        target,
		"network_profile":               profile,
		"simulator_duration_ms":         duration.Milliseconds(),
		"simulator_simulated_delay_ms":  simulatedDelayMS,
	})
}

func writeJSONError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   code,
		"message": message,
	})
}

func logAccess(
	r *http.Request,
	target string,
	targetURL string,
	profile string,
	statusCode int,
	duration time.Duration,
	simulatedDelayMS int,
	err error,
) {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = r.Header.Get("X-Correlation-ID")
	}

	entry := AccessLog{
		RequestID:        requestID,
		Method:           r.Method,
		Path:             r.URL.Path,
		Target:           target,
		TargetURL:        targetURL,
		NetworkProfile:   profile,
		StatusCode:        statusCode,
		DurationMS:       duration.Milliseconds(),
		SimulatedDelayMS: simulatedDelayMS,
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
	}

	if err != nil {
		entry.Error = err.Error()
	}

	raw, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		log.Printf("simulator_log_marshal_error: %v", marshalErr)
		return
	}

	log.Printf("simulator_access_log=%s", string(raw))
}