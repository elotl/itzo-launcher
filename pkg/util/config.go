package util

import "strings"

const ItzoFlagPrefix = "itzoFlag"
var DefaultItzoFlags = []string{"--v", "5"}

func GetItzoFlags(config map[string]string) []string {
	if config == nil {
		return []string{}
	}
	itzoFlags := make([]string, 0)
	addDefaults := true
	for key, value := range config {
		if strings.HasPrefix(key, ItzoFlagPrefix) {
			flagName := strings.Replace(key, ItzoFlagPrefix, "", 1)
			if flagName == "-v" || flagName == "--v" {
				addDefaults = false
			}
			itzoFlags = append(itzoFlags, flagName, value)
		}
	}
	if addDefaults {
		itzoFlags = append(itzoFlags, DefaultItzoFlags...)
	}
	return itzoFlags
}

