package content

import (
	"context"
	"time"

	"github.com/applicaset/pkg/ref"
)

const MaxTitleLength = 200

type Post struct {
	ID string
	// AuthorRef points at a user in another service. This service never dereferences it.
	AuthorRef   string
	Title       string
	Body        string
	ContentType string
	Status      Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
	// PublishedAt is set the first time a post is published and never moved afterwards.
	PublishedAt *time.Time
}

type PostRepository interface {
	Insert(ctx context.Context, post *Post) error
	Update(ctx context.Context, post *Post) error
	Delete(ctx context.Context, id string) error
	Get(ctx context.Context, id string) (*Post, error)
	List(ctx context.Context, filter PostFilter) ([]Post, error)
}

// PostFilter narrows a listing. A zero value lists everything up to the default limit.
type PostFilter struct {
	// Status is empty to match any status.
	Status Status
	// AuthorRef is empty to match any author.
	AuthorRef string
	Limit     int
}

func (p *Post) Ref() string {
	return ref.MustNew(ServiceName, PostResourceType, p.ID).String()
}
