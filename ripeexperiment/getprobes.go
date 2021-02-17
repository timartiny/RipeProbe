package ripeexperiment

import (
	atlas "github.com/keltia/ripe-atlas"
)

// ProbeIPs struct will store information important for future experiments only.
type ProbeIPs struct {
	ID        int    `json:"id"`
	AddressV4 string `json:"address_v4"`
	PrefixV4  string `json:"prefix_v4"`
	AddressV6 string `json:"address_v6"`
	PrefixV6  string `json:"prefix_v6"`
}

// GetProbes uses the ripe-atlas client to get online probes from a given country code
func GetProbes(countryCode string) []atlas.Probe {
	client, err := atlas.NewClient(atlas.Config{})
	if err != nil {
		errorLogger.Fatalf("Error creating atlas client, err: %v\n", err)
	}
	opts := make(map[string]string)
	opts["country_code"] = countryCode
	opts["status"] = "1"
	probes, err := client.GetProbes(opts)
	if err != nil {
		errorLogger.Fatalf("Error getting probes, err: %v\n", err)
	}
	return probes
}
