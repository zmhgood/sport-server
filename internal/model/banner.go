package model

const HomeBannersConfigKey = "home_banners"

// Banner order is the order of items in the saved configuration.
type Banner struct {
	ID          uint64 `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
	Enabled     bool   `json:"enabled"`
}

type BannerConfig struct {
	Configured bool     `json:"configured"`
	Items      []Banner `json:"items"`
}
