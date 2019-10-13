package turtleware

import (
	"github.com/sirupsen/logrus"

	"encoding/xml"
	"net/http"
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
func WriteError(w http.ResponseWriter, r *http.Request, code int, errors ...error) {
	w.WriteHeader(code)
	w.Header().Add("Cache-Control", "no-store")

	if r.Method != http.MethodHead {
		fields := logrus.Fields{
			"errors":     errors,
			"error_code": code,
		}
		logrus.WithFields(fields).Warnf("Writing errors: %s", errors)

		errorList := make(errorList, len(errors))
		for i, err := range errors {
			errorList[i] = err.Error()
		}

		errorMap := errorResponse{
			Status: code,
			Text:   http.StatusText(code),
			Errors: errorList,
		}

		defer func() {
			if r := recover(); r != nil {
				logrus.WithFields(fields).Errorf("Error while marshalling error message: %s", r)
			}
		}()
		EmissioneWriter.Write(w, r, code, errorMap)
	}
}
