package model

type SearchResult struct {
	Query string       `json:"query"`
	Items []SearchItem `json:"items"`
}

type SearchItem struct {
	Type     string `json:"type"`
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	URL      string `json:"url"`
}
