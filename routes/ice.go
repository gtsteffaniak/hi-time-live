package routes

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
)

// ICEServer mirrors the browser's RTCIceServer. URLs is raw JSON so an operator
// can pass either a single URL string or an array through ICE_SERVERS.
type ICEServer struct {
	URLs       json.RawMessage `json:"urls"`
	Username   string          `json:"username,omitempty"`
	Credential string          `json:"credential,omitempty"`
}

// iceServerURLs normalizes an RTCIceServer's `urls` value — either a single URL
// string or an array of them — into the list the browser will use. Anything
// that is neither, or that yields no non-empty URL, is unusable.
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

// defaultSTUNServers is used when nothing is configured. Only hosts that still
// answer are kept: the list this replaced included the long-dead
// stun.stunprotocol.org and several stale consumer-VoIP hosts, which added
// gathering latency without ever contributing a usable candidate.
//
// STUN alone cannot traverse symmetric NAT or a restrictive firewall, so a
// deployment that must reach those users needs a TURN relay (see TURN_URL).
var defaultSTUNServers = []ICEServer{
	{URLs: json.RawMessage(`"stun:stun.l.google.com:19302"`)},
	{URLs: json.RawMessage(`"stun:stun1.l.google.com:19302"`)},
	{URLs: json.RawMessage(`"stun:stun2.l.google.com:19302"`)},
	{URLs: json.RawMessage(`"stun:stun3.l.google.com:19302"`)},
	{URLs: json.RawMessage(`"stun:stun4.l.google.com:19302"`)},
}

// iceServersJSON is the marshalled ICE server list rendered into the room page.
var iceServersJSON template.JS

// ConfigureICE builds the ICE server list handed to browsers. ICE_SERVERS, when
// set, is a full override; it is re-marshalled so only the documented fields
// reach the page. Otherwise the STUN defaults are used and, when TURN_URL is
// set, a TURN entry is appended.
func ConfigureICE(iceServersRaw, turnURL, turnUsername, turnCredential string) error {
	if strings.TrimSpace(iceServersRaw) != "" {
		var parsed []ICEServer
		if err := json.Unmarshal([]byte(iceServersRaw), &parsed); err != nil {
			return fmt.Errorf("ICE_SERVERS must be a JSON array of RTCIceServer objects: %w", err)
		}
		// The browser rejects a server with no usable URL (or an empty URL
		// list), so a malformed override would only surface later as a room
		// that cannot connect. Reject it here, where the operator can see why.
		for i, server := range parsed {
			if len(iceServerURLs(server.URLs)) == 0 {
				return fmt.Errorf("ICE_SERVERS[%d] must have a non-empty urls string or array", i)
			}
		}
		raw, err := json.Marshal(parsed)
		if err != nil {
			return err
		}
		iceServersJSON = template.JS(raw)
		return nil
	}

	servers := append([]ICEServer{}, defaultSTUNServers...)
	if turnURL = strings.TrimSpace(turnURL); turnURL != "" {
		var urls []string
		for _, u := range strings.Split(turnURL, ",") {
			if u = strings.TrimSpace(u); u != "" {
				urls = append(urls, u)
			}
		}
		if len(urls) > 0 {
			rawURLs, err := json.Marshal(urls)
			if err != nil {
				return err
			}
			servers = append(servers, ICEServer{
				URLs:       rawURLs,
				Username:   turnUsername,
				Credential: turnCredential,
			})
		}
	}

	raw, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	iceServersJSON = template.JS(raw)
	return nil
}

// iceServersForPage returns the ICE server JSON for the room page, falling back
// to the STUN defaults when ConfigureICE has not run (as in unit tests).
func iceServersForPage() template.JS {
	if len(iceServersJSON) > 0 {
		return iceServersJSON
	}
	raw, err := json.Marshal(defaultSTUNServers)
	if err != nil {
		return template.JS("[]")
	}
	return template.JS(raw)
}
