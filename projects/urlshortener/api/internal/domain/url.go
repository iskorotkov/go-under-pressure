package domain

type CreateURLRequest struct {
	URL string `json:"url"`
}

type CreateURLResponse struct {
	ShortCode   string `json:"short_code"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type CreateURLBatchRequest struct {
	URLs []string `json:"urls"`
}

type CreateURLBatchResponse struct {
	URLs []CreateURLResponse `json:"urls"`
}
