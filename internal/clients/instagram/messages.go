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

type recipient struct {
	ID        string `json:"id,omitempty"`
	CommentID string `json:"comment_id,omitempty"`
}

type message struct {
	Text string `json:"text"`
}

type privateReplyReq struct {
	Recipient recipient `json:"recipient"`
	Message   message   `json:"message"`
}

type sendMessageReq struct {
	Recipient recipient `json:"recipient"`
	Message   message   `json:"message"`
}

type messageRes struct {
	MessageID   string `json:"message_id"`
	RecipientID string `json:"recipient_id"`
}

// Message contains the platform confirmation, without workflow state.
type Message struct {
	ID          string
	RecipientID string
}

func newPrivateReplyReq(commentID, text string) (io.Reader, error) {
	enc, err := json.Marshal(&privateReplyReq{
		Recipient: recipient{CommentID: commentID},
		Message:   message{Text: text},
	})
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	return bytes.NewBuffer(enc), nil
}

func newSendMessageReq(contactID, text string) (io.Reader, error) {
	enc, err := json.Marshal(&sendMessageReq{
		Recipient: recipient{ID: contactID},
		Message:   message{Text: text},
	})
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	return bytes.NewBuffer(enc), nil
}

// PrivateReply starts a private conversation using the original comment ID.
// Recipient matching belongs to the workflow consuming the returned message.
func (c *Client) PrivateReply(
	ctx context.Context,
	accountID string,
	commentID string,
	contactID string,
	text string,
) (Message, error) {
	body, err := newPrivateReplyReq(commentID, text)
	if err != nil {
		return Message{}, errtrace.Wrap(err)
	}

	response, err := c.httpClient.
		Post("/%s/messages", url.PathEscape(accountID)).
		SetRequestBody(body).
		Execute(ctx)
	if err != nil {
		return Message{}, errtrace.Wrap(err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		var res messageRes
		if err := json.NewDecoder(response.Body).Decode(&res); err != nil {
			return Message{}, errtrace.Wrap(err)
		}

		return Message{
			ID:          res.MessageID,
			RecipientID: res.RecipientID,
		}, nil

	default:
		return Message{}, errtrace.Errorf(
			"%w: instagram private reply status %d",
			errs.ErrUnexpectedResponse,
			response.StatusCode,
		)
	}
}

// SendMessage addresses an existing conversation by the scoped contact ID.
func (c *Client) SendMessage(
	ctx context.Context,
	accountID string,
	contactID string,
	text string,
) (Message, error) {
	body, err := newSendMessageReq(contactID, text)
	if err != nil {
		return Message{}, errtrace.Wrap(err)
	}

	response, err := c.httpClient.
		Post("/%s/messages", url.PathEscape(accountID)).
		SetRequestBody(body).
		Execute(ctx)
	if err != nil {
		return Message{}, errtrace.Wrap(err)
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusOK:
		var res messageRes
		if err := json.NewDecoder(response.Body).Decode(&res); err != nil {
			return Message{}, errtrace.Wrap(err)
		}

		return Message{
			ID:          res.MessageID,
			RecipientID: res.RecipientID,
		}, nil

	default:
		return Message{}, errtrace.Errorf(
			"%w: instagram message status %d",
			errs.ErrUnexpectedResponse,
			response.StatusCode,
		)
	}
}
