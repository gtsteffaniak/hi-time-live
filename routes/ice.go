package routes

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ICEServer mirrors the browser's RTCIceServer. URLs is raw JSON so an operator
// can pass either a single URL string or an array through ICE_SERVERS.
type ICEServer struct {
	URLs       json.RawMessage `json:"urls"`
	Username   string          `json:"username,omitempty"`
	Credential string          `json:"credential,omitempty"`
}

type iceConfig struct {
	devMode            bool
	overrideRaw        string
	turnURLs           []string
	turnUsername       string
	turnCredential     string
	turnRESTSecret     string
	turnRESTTTL        time.Duration
	turnRESTUserSuffix string
}

var activeICE iceConfig

// iceServerURLs normalizes an RTCIceServer's `urls` value — either a single URL
// string or an array of them — into the list the browser will use.
func iceServerURLs(raw json.RawMessage) []string {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if strings.TrimSpace(single) == "" {
			return nil
		}
		return []string{single}
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil
	}
	var urls []string
	for _, u := range many {
		if strings.TrimSpace(u) != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

func validICEURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch u.Scheme {
	case "stun", "stuns", "turn", "turns":
		return true
	default:
		return false
	}
}

func validateICEServerURLs(servers []ICEServer) error {
	for i, server := range servers {
		for j, iceURL := range iceServerURLs(server.URLs) {
			if !validICEURL(iceURL) {
				return fmt.Errorf("ICE_SERVERS[%d].urls[%d] %q must use the stun, stuns, turn, or turns scheme", i, j, iceURL)
			}
		}
	}
	return nil
}

// defaultSTUNServers is used when nothing is configured.
var defaultSTUNServers = []ICEServer{
	{URLs: json.RawMessage(`"stun:stun.l.google.com:19302"`)},
	{URLs: json.RawMessage(`"stun:stun1.l.google.com:19302"`)},
	{URLs: json.RawMessage(`"stun:stun2.l.google.com:19302"`)},
	{URLs: json.RawMessage(`"stun:stun3.l.google.com:19302"`)},
	{URLs: json.RawMessage(`"stun:stun4.l.google.com:19302"`)},
}

func configuredICEOverrides(iceServersRaw, turnURL, turnUsername, turnCredential, turnRESTSecret string) string {
	var set []string
	for _, setting := range []struct {
		name  string
		value string
	}{
		{"ICE_SERVERS", iceServersRaw},
		{"TURN_URL", turnURL},
		{"TURN_USERNAME", turnUsername},
		{"TURN_CREDENTIAL", turnCredential},
		{"TURN_REST_SECRET", turnRESTSecret},
	} {
		if strings.TrimSpace(setting.value) != "" {
			set = append(set, setting.name)
		}
	}
	return strings.Join(set, ", ")
}

// turnRESTCredential builds a coturn-compatible time-limited username and
// password: base64(HMAC-SHA1(secret, username)) with username = expiry[:suffix].
func turnRESTCredential(secret string, ttl time.Duration, userSuffix string) (username, credential string) {
	expiry := time.Now().Add(ttl).Unix()
	username = strconv.FormatInt(expiry, 10)
	if strings.TrimSpace(userSuffix) != "" {
		username = username + ":" + strings.TrimSpace(userSuffix)
	}
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(username))
	credential = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return username, credential
}

// ConfigureICE stores ICE/TURN settings for room pages.
//
// ICE_SERVERS replaces the built-in list entirely (development only when it
// includes relay credentials). TURN_URL adds a relay alongside the default STUN
// servers. Static TURN_USERNAME/TURN_CREDENTIAL are development-only because
// they are embedded in every room page. TURN_REST_SECRET enables coturn-style
// time-limited credentials that are minted per page load without publishing the
// shared secret.
func ConfigureICE(devMode bool, iceServersRaw, turnURL, turnUsername, turnCredential, turnRESTSecret string, turnRESTTTL time.Duration, turnRESTUserSuffix string) error {
	activeICE = iceConfig{
		devMode:            devMode,
		overrideRaw:        strings.TrimSpace(iceServersRaw),
		turnUsername:       turnUsername,
		turnCredential:     turnCredential,
		turnRESTSecret:     strings.TrimSpace(turnRESTSecret),
		turnRESTTTL:        turnRESTTTL,
		turnRESTUserSuffix: turnRESTUserSuffix,
	}

	for _, u := range strings.Split(turnURL, ",") {
		if u = strings.TrimSpace(u); u != "" {
			if !validICEURL(u) {
				return fmt.Errorf("TURN_URL entry %q must use the stun, stuns, turn, or turns scheme", u)
			}
			activeICE.turnURLs = append(activeICE.turnURLs, u)
		}
	}

	staticTurn := len(activeICE.turnURLs) > 0 &&
		(strings.TrimSpace(turnUsername) != "" || strings.TrimSpace(turnCredential) != "")
	restTurn := len(activeICE.turnURLs) > 0 && activeICE.turnRESTSecret != ""

	if !devMode {
		ignored := configuredICEOverrides(iceServersRaw, turnURL, turnUsername, turnCredential, "")
		if activeICE.overrideRaw != "" || staticTurn {
			if ignored != "" {
				slog.Warn("ignoring ICE/TURN settings that would publish long-lived relay credentials outside dev mode",
					"settings", ignored)
			}
			activeICE.overrideRaw = ""
			activeICE.turnUsername = ""
			activeICE.turnCredential = ""
		}
		if restTurn {
			// REST credentials are minted per page; the shared secret stays on the server.
			return validateRESTConfig()
		}
		if activeICE.turnRESTSecret != "" {
			return validateRESTConfig()
		}
		if ignored != "" && !restTurn {
			slog.Warn("ignoring ICE/TURN configuration outside dev mode; serving the built-in STUN servers",
				"settings", ignored)
		}
		activeICE.turnURLs = nil
		return nil
	}

	if activeICE.overrideRaw != "" {
		var parsed []ICEServer
		if err := json.Unmarshal([]byte(activeICE.overrideRaw), &parsed); err != nil {
			return fmt.Errorf("ICE_SERVERS must be a JSON array of RTCIceServer objects: %w", err)
		}
		for i, server := range parsed {
			if len(iceServerURLs(server.URLs)) == 0 {
				return fmt.Errorf("ICE_SERVERS[%d] must have a non-empty urls string or array", i)
			}
		}
		if err := validateICEServerURLs(parsed); err != nil {
			return err
		}
		return nil
	}

	if len(activeICE.turnURLs) > 0 && activeICE.turnRESTSecret == "" &&
		(strings.TrimSpace(turnUsername) == "" || strings.TrimSpace(turnCredential) == "") {
		return fmt.Errorf("TURN_URL requires both TURN_USERNAME and TURN_CREDENTIAL in dev mode when TURN_REST_SECRET is not set")
	}

	if restTurn {
		return validateRESTConfig()
	}
	if activeICE.turnRESTSecret != "" {
		return validateRESTConfig()
	}
	return nil
}

func validateRESTConfig() error {
	if len(activeICE.turnURLs) == 0 {
		return fmt.Errorf("TURN_REST_SECRET requires TURN_URL")
	}
	if activeICE.turnRESTTTL <= 0 {
		activeICE.turnRESTTTL = 24 * time.Hour
	}
	return nil
}

func buildICEServers() ([]ICEServer, error) {
	if activeICE.overrideRaw != "" && activeICE.devMode {
		var parsed []ICEServer
		if err := json.Unmarshal([]byte(activeICE.overrideRaw), &parsed); err != nil {
			return nil, err
		}
		return parsed, nil
	}

	servers := append([]ICEServer{}, defaultSTUNServers...)
	if len(activeICE.turnURLs) == 0 {
		return servers, nil
	}

	rawURLs, err := json.Marshal(activeICE.turnURLs)
	if err != nil {
		return nil, err
	}

	username := activeICE.turnUsername
	credential := activeICE.turnCredential
	if activeICE.turnRESTSecret != "" {
		username, credential = turnRESTCredential(activeICE.turnRESTSecret, activeICE.turnRESTTTL, activeICE.turnRESTUserSuffix)
	}

	servers = append(servers, ICEServer{
		URLs:       rawURLs,
		Username:   username,
		Credential: credential,
	})
	return servers, nil
}

// iceServersForPage returns JSON for the room page's #ice-config element.
func iceServersForPage() template.JS {
	servers, err := buildICEServers()
	if err != nil {
		slog.Error("building ICE server list", "error", err)
		servers = defaultSTUNServers
	}
	raw, err := json.Marshal(servers)
	if err != nil {
		return template.JS("[]")
	}
	return template.JS(raw)
}

// turnRESTEnabled reports whether room pages should refresh ICE credentials.
func turnRESTEnabled() bool {
	return activeICE.turnRESTSecret != "" && len(activeICE.turnURLs) > 0
}

// iceConfigHandler returns a fresh ICE server list for long-lived room tabs.
func iceConfigHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if !validCode(code) {
		http.Error(w, "Invalid room code", http.StatusBadRequest)
		return
	}
	servers, err := buildICEServers()
	if err != nil {
		http.Error(w, "Could not build ICE configuration", http.StatusInternalServerError)
		return
	}
	raw, err := json.Marshal(servers)
	if err != nil {
		http.Error(w, "Could not encode ICE configuration", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}
