package results

//Result stores actual result of query, stored in abuf, needs to be decoded.
type Result struct {
	Rt      float64 `json:"rt,omitempty"`
	Size    int     `json:"size,omitempty"`
	Abuf    string  `json:"abuf,omitempty"`
	ID      int     `json:"id,omitempty"`
	ANcount int     `json:"ANCOUNT,omitempty"`
	QDcount int     `json:"QDCOUNT,omitempty"`
	NScount int     `json:"NSCOUNT,omitempty"`
	ARcount int     `json:"ARCOUNT,omitmpty"`
}

//ResultSet is the wrapper for results, store metadata with result
type ResultSet struct {
	Time     int    `json:"time,omitempty"`
	Lts      int    `json:"lts,omitempty"`
	SubID    int    `json:"subid,omitempty"`
	SubMax   int    `json:"submax,omitempty"`
	DestAddr string `json:"dst_addr,omitempty"`
	AF       int    `json:"af,omitempty"`
	SrcAddr  string `json:"src_addr,omitempty"`
	Proto    string `json:"proto,omitempty"`
	Result   Result `json:"result,omitempty"`
}

//MeasurementResult is the wrapper for a measurement's results
type MeasurementResult struct {
	Fw              int                    `json:"fw,omitempty"`
	Lts             int                    `json:"lts,omitempty"`
	DestAddr        string                 `json:"dst_addr,omitempty"`
	AF              int                    `json:"af,omitempty"`
	ResultSet       []ResultSet            `json:"resultset,omitempty"`
	Result          Result                 `json:"result,omitempty"`
	Error           map[string]interface{} `json:"error,omitempty"`
	MsmID           int                    `json:"msm_id,omitempty"`
	PrbID           int                    `json:"prb_id,omitempty"`
	Timestamp       int                    `json:"timestamp,omitempty"`
	MsmName         string                 `json:"msm_name,omitempty"`
	From            string                 `json:"from,omitempty"`
	Type            string                 `json:"type,omitempty"`
	GroupID         int                    `json:"group_id,omitempty"`
	StoredTimestamp int                    `json:"stored_timestamp,omitempty"`
}

//WhiteboardResult is the wrapper for important info for one measurement
type WhiteboardResult struct {
	ProbeID    int                 `json:"probe_id,omitempty"`
	ProbeIP    string              `json:"probe_ip,omitempty"`
	ResolverIP string              `json:"resolver_ip,omitempty"`
	Queries    map[string][]string `json:"queries,omitempty"`
}

type ProbeResult struct {
	ProbeID int           `json:"probe_id,omitempty"`
	V4Addr  string        `json:"v4_address,omitempty"`
	V6Addr  string        `json:"v6_address,omitempty"`
	V4ToV4  []QueryResult `json:"v4_to_v4,omitempty"`
	V4ToV6  []QueryResult `json:"v4_to_v6,omitempty"`
	V6ToV4  []QueryResult `json:"v6_to_v4,omitempty"`
	V6ToV6  []QueryResult `json:"v6_to_v6,omitempty"`
}

type QueryResult struct {
	ResolverIP   string              `json:"resolver_ip"`
	ResolverType string              `json:"resolver_type"`
	Queries      map[string][]string `json:"queries,omitempty"`
}
