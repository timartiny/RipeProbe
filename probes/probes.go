package probes

// ProbeIP struct will store information to let probes be usable easily
type ProbeIP struct {
	ID          int    `json:"id"`
	AddressV4   string `json:"address_v4"`
	PrefixV4    string `json:"prefix_v4"`
	AddressV6   string `json:"address_v6"`
	PrefixV6    string `json:"prefix_v6"`
	CountryCode string `json:"country_code"`
}
