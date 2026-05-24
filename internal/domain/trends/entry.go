package trends

type TrendEntry struct {
	Query string `json:"query"`
	Count int64  `json:"count"`
}
