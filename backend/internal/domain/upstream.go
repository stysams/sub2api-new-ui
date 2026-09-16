package domain

const (
	UpstreamKindNewAPI        = "newapi"
	UpstreamKindSub2API       = "sub2api"
	UpstreamResourceTypeToken = "token"
	UpstreamResourceTypeKey   = "key"
)

type UpstreamGroupItem struct {
	ID       string   `json:"id,omitempty"`
	Name     string   `json:"name"`
	Ratio    *float64 `json:"ratio,omitempty"`
	Desc     string   `json:"desc,omitempty"`
	Platform string   `json:"platform,omitempty"`
}

type UpstreamBalanceSnapshot struct {
	Quota        float64  `json:"quota"`
	UsedQuota    float64  `json:"used_quota"`
	Remaining    *float64 `json:"remaining,omitempty"`
	RequestCount int64    `json:"request_count"`
	Currency     string   `json:"currency,omitempty"`
	FetchedAt    string   `json:"fetched_at"`
}

type UpstreamGroupSnapshot struct {
	Items     []UpstreamGroupItem `json:"items"`
	FetchedAt string              `json:"fetched_at"`
}
