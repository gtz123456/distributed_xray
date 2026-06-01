package node

import (
	"regexp"
	"strings"

	"github.com/oneclickvirt/UnlockTests/executor"
	"github.com/oneclickvirt/UnlockTests/utils"
	"github.com/oneclickvirt/defaultset"
)

func getMediaConnectivity(language string) string {
	var res string
	readStatus := executor.ReadSelect(language, "20")
	if !readStatus {
		return ""
	}
	if executor.IPV4 {
		res += defaultset.Blue("IPV4:") + "\n"
		res += executor.RunTests(utils.Ipv4HttpClient, "ipv4", language, false)
		return res
	}
	if executor.IPV6 {
		res += defaultset.Blue("IPV6:") + "\n"
		res += executor.RunTests(utils.Ipv6HttpClient, "ipv6", language, false)
		return res
	}
	return ""
}

func parseConnectivity(output string, connectivity map[string]bool) map[string]bool {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	clean := re.ReplaceAllString(output, "")

	lines := strings.Split(clean, "\n")

	mapping := map[string]string{
		"BingSearch":        "Bing",
		"GoogleSearch":      "Google",
		"Claude":            "Claude",
		"ChatGPT":           "ChatGPT",
		"Gemini":            "Gemini",
		"MetaAI":            "MetaAI",
		"YouTube Region":    "Youtube",
		"Netflix":           "Netflix",
		"Disney+":           "DisneyPlus",
		"Amazon Prime Video": "AmazonPrime",
		"TikTok":            "TikTok",
		"Niconico":          "Niconico",
		"Wavve":             "Wavve",
		"Peacock TV":        "PeacockTV",
		"Discovery+":        "DiscoveryPlus",
		"Hulu":              "Hulu",
		"Crunchyroll":       "Crunchyroll",
		"NBA TV":            "NBATV",
		"Google Play Store": "GooglePlayStore",
		"Steam Store":       "Steam",
		"Spotify":           "Spotify", // Can be 'Spotify Registration'
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		for k, v := range mapping {
			if strings.HasPrefix(line, k) {
				if strings.Contains(line, "YES") {
					connectivity[v] = true
				} else if !connectivity[v] {
					// Only set to false if it's not already true.
					// This prevents lines like "Netflix CDN" from overwriting the "Netflix" YES result.
					connectivity[v] = false
				}
			}
		}
	}

	return connectivity
}

func GetConnectivity() map[string]bool {
	connectivityStr := getMediaConnectivity("en")

	if connectivityStr == "" {
		return nil
	}

	connectivity := map[string]bool{
		"Bing":            false,
		"Google":          false,
		"Claude":          false,
		"ChatGPT":         false,
		"Gemini":          false,
		"MetaAI":          false,
		"Youtube":         false,
		"Netflix":         false,
		"DisneyPlus":      false,
		"AmazonPrime":     false,
		"TikTok":          false,
		"Niconico":        false,
		"Wavve":           false,
		"PeacockTV":       false,
		"DiscoveryPlus":   false,
		"Hulu":            false,
		"Crunchyroll":     false,
		"NBATV":           false,
		"GooglePlayStore": false,
		"Steam":           false,
		"Spotify":         false,
	}

	return parseConnectivity(connectivityStr, connectivity)
}
