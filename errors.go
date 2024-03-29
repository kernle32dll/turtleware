package turtleware

import (
	"github.com/go-logr/logr"

	"context"
	"encoding/xml"
	"log/slog"
	"net/http"
	"strings"
)

type errorList []string

func (errorList errorList) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	tokens := []xml.Token{start}

	for _, value := range errorList {
		t := xml.StartElement{Name: xml.Name{Local: "Error"}}
		tokens = append(tokens, t, xml.CharData(value), t.End())
	}

	tokens = append(tokens, start.End())

	for _, t := range tokens {
		err := e.EncodeToken(t)
		if err != nil {
			return err
		}
	}

	// flush to ensure tokens are written
	err := e.Flush()
	if err != nil {
		return err
	}

	return nil
}

type errorResponse struct {
	XMLName xml.Name  `xml:"ErrorResponse" json:"-"`
	Status  int       `json:"status" xml:"Status"`
	Text    string    `json:"text" xml:"Text"`
	Errors  errorList `json:"errors" xml:"ErrorList"`
}

// WriteError sets the given status code, and writes a nicely formatted json
// errors to the response body - if the request type is not HEAD.
func WriteError(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	code int,
	errors ...error,
) {
	errList := make(errorList, len(errors))
	for i, err := range errors {
		errList[i] = err.Error()
	}

	for _, err := range errors {
		// nolint errcheck: Returned error is not checked, as its just err as passed in
		_ = TagContextSpanWithError(ctx, err)
	}

	w.Header().Set("Cache-Control", "no-store")

	if r.Method != http.MethodHead {
		logger := slog.New(logr.ToSlogHandler(logr.FromContextOrDiscard(ctx))).With(
			slog.String("error", strings.Join(errList, ", ")),
			slog.Int("error_code", code),
		)
		logger.WarnContext(ctx, "Writing errors")

		errorMap := errorResponse{
			Status: code,
			Text:   http.StatusText(code),
			Errors: errList,
		}

		defer func() {
			if r := recover(); r != nil {
				w.WriteHeader(http.StatusInternalServerError)
				logger.ErrorContext(
					ctx,
					"Error while marshalling error message",
					slog.Any("error", r),
				)
			}
		}()
		EmissioneWriter.Write(w, r, code, errorMap)
	} else {
		// No body, but we still require the status code
		w.WriteHeader(code)
	}
}
