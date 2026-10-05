package remote

import (
	"context"

	"github.com/applicaset/blog/web"
	discussclient "github.com/applicaset/discuss/client"
	"github.com/applicaset/pkg/api/discussapi"
)

// Discuss satisfies web.Discuss by calling the discussion service.
type Discuss struct {
	client *discussclient.Client
}

var _ web.Discuss = (*Discuss)(nil)

func NewDiscuss(client *discussclient.Client) *Discuss {
	return &Discuss{client: client}
}

func (d *Discuss) ListComments(ctx context.Context, resourceRef string) ([]web.Comment, error) {
	comments, err := d.client.ListComments(ctx, resourceRef)
	if err != nil {
		return nil, translate(err)
	}

	converted := make([]web.Comment, 0, len(comments))
	for _, comment := range comments {
		converted = append(converted, *toWebComment(comment))
	}

	return converted, nil
}

func (d *Discuss) AddComment(
	ctx context.Context,
	actorRef, resourceRef, parentID, body string,
) (*web.Comment, error) {
	comment, err := d.client.AddComment(ctx, actorRef, resourceRef, parentID, body)
	if err != nil {
		return nil, translate(err)
	}

	return toWebComment(comment), nil
}

func (d *Discuss) EditComment(
	ctx context.Context,
	actorRef, resourceRef, commentID string,
	revision int,
	body string,
) error {
	_, err := d.client.EditComment(ctx, actorRef, resourceRef, commentID, revision, body)

	return translate(err)
}

func (d *Discuss) DeleteComment(
	ctx context.Context,
	actorRef, resourceRef, commentID string,
) error {
	_, err := d.client.DeleteComment(ctx, actorRef, resourceRef, commentID)

	return translate(err)
}

func (d *Discuss) PurgeResource(ctx context.Context, resourceRef string) error {
	return translate(d.client.PurgeResource(ctx, resourceRef))
}

func toWebComment(comment discussapi.Comment) *web.Comment {
	return &web.Comment{
		ID:        comment.ID,
		ParentID:  comment.ParentID,
		AuthorRef: comment.AuthorRef,
		Body:      comment.Body,
		Revision:  comment.Revision,
		CreatedAt: comment.CreatedAt,
		EditedAt:  comment.EditedAt,
		DeletedAt: comment.DeletedAt,
	}
}
