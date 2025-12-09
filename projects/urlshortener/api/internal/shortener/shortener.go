package shortener

import "github.com/sqids/sqids-go"

type Shortener struct {
	sqids *sqids.Sqids
}

func New() (*Shortener, error) {
	s, err := sqids.New(sqids.Options{
		MinLength: 6,
	})
	if err != nil {
		return nil, err
	}
	return &Shortener{sqids: s}, nil
}

func (s *Shortener) Generate(id uint) (string, error) {
	return s.sqids.Encode([]uint64{uint64(id)})
}
