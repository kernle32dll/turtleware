package turtleware_test

import (
	"github.com/google/uuid"
	"github.com/justinas/alice"
	"github.com/kernle32dll/turtleware"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/stretchr/testify/suite"

	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
)

type CommonSuite struct {
	suite.Suite

	entityUUID uuid.UUID
	userUUID   uuid.UUID
}

func (s *CommonSuite) SetupTest() {
	s.entityUUID = uuid.New()
	s.userUUID = uuid.New()
}

func (s *CommonSuite) buildEntityUUIDChain(h http.Handler) http.Handler {
	return turtleware.EntityUUIDMiddleware(func(r *http.Request) (uuid.UUID, error) {
		return s.entityUUID, nil
	})(h)
}

func (s *CommonSuite) buildAuthChain(h http.Handler) http.Handler {
	s.T().Helper()

	privateKey, err := jwk.FromRaw([]byte("secret-passphrase"))
	s.Require().NoError(err)
	s.Require().NoError(privateKey.Set(jwk.KeyIDKey, "super-key"))
	s.Require().NoError(privateKey.Set(jwk.AlgorithmKey, jwa.HS512))

	keySet := jwk.NewSet()
	s.Require().NoError(keySet.AddKey(privateKey))

	return alice.New(
		func(handler http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				userUUIDString := s.userUUID.String()
				if s.userUUID == uuid.Nil {
					userUUIDString = ""
				}

				token := s.generateToken(
					jwa.HS512,
					privateKey,
					map[string]interface{}{"uuid": userUUIDString},
					map[string]interface{}{jwk.KeyIDKey: privateKey.KeyID()},
				)

				r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
				handler.ServeHTTP(w, r)
			})
		},
		turtleware.AuthBearerHeaderMiddleware,
		turtleware.AuthClaimsMiddleware(keySet),
	).Then(h)
}

func (s *CommonSuite) generateToken(
	algo jwa.SignatureAlgorithm,
	key interface{},
	claims map[string]interface{},
	headers map[string]interface{},
) string {
	t := jwt.New()

	for k, v := range claims {
		if err := t.Set(k, v); err != nil {
			s.Require().NoError(err)
		}
	}

	hdr := jws.NewHeaders()
	for k, v := range headers {
		if err := hdr.Set(k, v); err != nil {
			s.Require().NoError(err)
		}
	}

	signedT, err := jwt.Sign(t, jwt.WithKey(algo, key, jws.WithProtectedHeaders(hdr)))
	if err != nil {
		s.Require().NoError(err)
	}

	return string(signedT)
}

func (s *CommonSuite) loadTestDataString(name string) string {
	bufBytes, err := io.ReadAll(s.loadTestData(name))
	if err != nil {
		s.Require().NoError(err)
	}

	return string(bufBytes)
}

func (s *CommonSuite) loadTestData(name string) io.Reader {
	filePath := path.Join("testdata", name)

	f, err := os.Open(filePath)
	if err != nil {
		s.Require().NoError(err)
	}

	s.T().Cleanup(func() {
		if err := f.Close(); err != nil && !strings.Contains(err.Error(), "file already closed") {
			s.T().Logf("Failed to close file handle for test data %q: %s", name, err)
		}
	})

	return f
}
