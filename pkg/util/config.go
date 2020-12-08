package util

import "strings"

const ItzoFlagPrefix = "itzoFlag"

func GetItzoFlags(config map[string]string) []string {
	itzoFlags := make([]string, 0)
	for key, value := range config {
		if strings.HasPrefix(key, ItzoFlagPrefix) {
			flagName := strings.Replace(key, ItzoFlagPrefix, "", 1)
			itzoFlags = append(itzoFlags, flagName, value)
		}
	}
	return itzoFlags
}

