package node

import (
	"regexp"
	"strings"

	"github.com/oneclickvirt/UnlockTests/utils"
	"github.com/oneclickvirt/UnlockTests/uts"
	"github.com/oneclickvirt/defaultset"
)

func getMediaConnectivity(language string) string {
	var res string
	readStatus := uts.ReadSelect(language, "0")
	if !readStatus {
		return ""
	}
	if uts.IPV4 {
		res += defaultset.Blue("IPV4:") + "\n"
		res += uts.RunTests(utils.Ipv4HttpClient, "ipv4", language, false)
		return res
	}
	if uts.IPV6 {
		res += defaultset.Blue("IPV6:") + "\n"
		res += uts.RunTests(utils.Ipv6HttpClient, "ipv6", language, false)
		return res
	}
	return ""
}

func parseConnectivity(output string, connectivity map[string]bool) map[string]bool {
	re := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	clean := re.ReplaceAllString(output, "")

	lines := strings.Split(clean, "\n")

	mapping := map[string]string{
		"BingSearch":   "Bing",
		"GoogleSearch": "Google",

		"Claude":  "Claude",
		"ChatGPT": "ChatGPT",
		"Gemini":  "Gemini",

		"YouTube Region": "Youtube",
		"Netflix":        "Netflix",
		"Disney+":        "DisneyPlus",
		"Spotify":        "Spotify",
		"TikTok":         "TikTok",

		"Reddit":      "Reddit",
		"Steam Store": "Steam",
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
				} else {
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
		"Bing":   false,
		"Google": false,

		"Claude":  false,
		"ChatGPT": false,
		"Gemini":  false,

		"Youtube":    false,
		"Netflix":    false,
		"DisneyPlus": false,
		"Spotify":    false,
		"TikTok":     false,

		"Reddit": false,

		"Steam": false,
	}

	return parseConnectivity(connectivityStr, connectivity)
}
