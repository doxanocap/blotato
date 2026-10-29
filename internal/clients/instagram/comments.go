package instagram

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"braces.dev/errtrace"

	"github.com/doxanocap/blotato/internal/domain/errs"
)

type commentReplyReq struct {
	Message string `json:"message"`
}

type commentReplyRes struct {
	ID string `json:"id"`
}

func newCommentReplyReq(text string) (io.Reader, error) {
	enc, err := json.Marshal(&commentReplyReq{
		Message: text,
	})
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	return bytes.NewBuffer(enc), nil
}

// ReplyComment posts a public reply to the triggering comment.
func (c *Client) ReplyComment(
	ctx context.Context,
	commentID string,
	text string,
) (string, error) {
	body, err := newCommentReplyReq(text)
	if err != nil {
		return "", errtrace.Wrap(err)
	}

	response, err := c.httpClient.
		Post("/%s/replies", url.PathEscape(commentID)).
		SetRequestBody(body).
		Execute(ctx)
	if err != nil {
		return "", errtrace.Wrap(err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		var res commentReplyRes
		if err := json.NewDecoder(response.Body).Decode(&res); err != nil {
			return "", errtrace.Wrap(err)
		}

		return res.ID, nil

	default:
		return "", errtrace.Errorf(
			"%w: instagram reply status %d",
			errs.ErrUnexpectedResponse,
			response.StatusCode,
		)
	}
}
