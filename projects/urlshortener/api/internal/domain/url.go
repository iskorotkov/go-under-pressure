package domain

import "time"

type URL struct {
	ID          uint      `gorm:"primaryKey"`
	ShortCode   string    `gorm:"size:16;uniqueIndex;not null"`
	OriginalURL string    `gorm:"type:text;not null"`
	CreatedAt   time.Time `gorm:"not null"`
	UpdatedAt   time.Time `gorm:"not null"`
}

type CreateURLRequest struct {
	URL string `json:"url" validate:"required,url"`
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
