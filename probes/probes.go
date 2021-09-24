package probes

import atlas "github.com/keltia/ripe-atlas"

// ProbeIP struct will store information to let probes be usable easily
type SimpleProbe struct {
	ID          int    `json:"id"`
	AddressV4   string `json:"address_v4"`
	PrefixV4    string `json:"prefix_v4"`
	AddressV6   string `json:"address_v6"`
	PrefixV6    string `json:"prefix_v6"`
	CountryCode string `json:"country_code"`
}

func AtlasProbeToSimpleProbe(probe atlas.Probe) SimpleProbe {
	var ret SimpleProbe
	ret.ID = probe.ID
	ret.AddressV4 = probe.AddressV4
	ret.PrefixV4 = probe.PrefixV4
	ret.AddressV6 = probe.AddressV6
	ret.PrefixV6 = probe.PrefixV6
	ret.CountryCode = probe.CountryCode

	return ret
}

func AtlasProbeSliceToSimpleProbeSlice(probeSlice []atlas.Probe) []SimpleProbe {
	var ret []SimpleProbe
	for _, probe := range probeSlice {
		ret = append(ret, AtlasProbeToSimpleProbe(probe))
	}

	return ret
}
