package turtleware_test

import (
	"github.com/google/uuid"
	"github.com/justinas/alice"
	"github.com/kernle32dll/turtleware"
	"github.com/stretchr/testify/suite"

	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type MiddlewareCommonSuite struct {
	CommonSuite

	response *httptest.ResponseRecorder
	request  *http.Request
}

func TestMiddlewareCommonSuite(t *testing.T) {
	suite.Run(t, &MiddlewareCommonSuite{})
}

func (s *MiddlewareCommonSuite) SetupTest() {
	s.CommonSuite.SetupTest()

	s.response = httptest.NewRecorder()
	s.request = httptest.NewRequest(http.MethodGet, "https://example.com/foo", http.NoBody)
}

func (s *MiddlewareCommonSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *MiddlewareCommonSuite) Test_DefaultErrorHandler_Handled() {
	// given
	cases := map[string]struct {
		err        error
		goldenFile string
		statusCode int
	}{
		"ErrResourceNotFound": {
			err:        turtleware.ErrResourceNotFound,
			goldenFile: "error_errresourcenotfound.json",
			statusCode: http.StatusNotFound,
		},
		"ErrMissingUserUUID": {
			err:        turtleware.ErrMissingUserUUID,
			goldenFile: "error_errmissinguseruuid.json",
			statusCode: http.StatusBadRequest,
		},
		"ErrMarshalling": {
			err:        turtleware.ErrMarshalling,
			goldenFile: "error_errmarshalling.json",
			statusCode: http.StatusBadRequest,
		},
		"ValidationWrapperError": {
			err: &turtleware.ValidationWrapperError{
				Errors: []error{
					errors.New("validation 1 error"),
					errors.New("validation 2 error"),
				},
			},
			goldenFile: "error_validationwrappererror.json",
			statusCode: http.StatusBadRequest,
		},
	}

	for testName, target := range cases {
		s.Run(testName, func() {
			// given
			targetError := target.err

			// when
			turtleware.DefaultErrorHandler(context.Background(), s.response, s.request, targetError)

			// then
			s.Equal(target.statusCode, s.response.Code)
			s.JSONEq(s.loadTestDataString("errorhandler/common/"+target.goldenFile), s.response.Body.String())
			s.True(turtleware.IsHandledByDefaultErrorHandler(targetError))
		})
	}
}

func (s *MiddlewareCommonSuite) Test_DefaultErrorHandler_NotHandled() {
	// given
	targetError := errors.New("some-error")

	// when
	turtleware.DefaultErrorHandler(context.Background(), s.response, s.request, targetError)

	// then
	s.JSONEq(s.loadTestDataString("errors/some_error.json"), s.response.Body.String())
	s.False(turtleware.IsHandledByDefaultErrorHandler(targetError))
}

func (s *MiddlewareCommonSuite) Test_EntityUUIDMiddleware_Success() {
	// given
	recordedUUID := uuid.Nil
	middlewareVerify := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		entityUUID, err := turtleware.EntityUUIDFromRequestContext(r.Context())
		s.Require().NoError(err)

		recordedUUID = entityUUID
	})

	expectedUUID := uuid.New()
	middleware := turtleware.EntityUUIDMiddleware(func(r *http.Request) (uuid.UUID, error) {
		return expectedUUID, nil
	})

	// when
	middleware(middlewareVerify).ServeHTTP(s.response, s.request)

	// then
	s.Empty(s.response.Body.String())
	s.Equal(expectedUUID, recordedUUID)
}

func (s *MiddlewareCommonSuite) Test_EntityUUIDMiddleware_Error() {
	// given
	middlewareVerify := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		s.Fail("unexpected middleware invocation")
	})

	middleware := turtleware.EntityUUIDMiddleware(func(r *http.Request) (uuid.UUID, error) {
		return uuid.Nil, errors.New("some-error")
	})

	// when
	middleware(middlewareVerify).ServeHTTP(s.response, s.request)

	// then
	s.Equal(http.StatusInternalServerError, s.response.Code)
	s.JSONEq(s.loadTestDataString("errors/some_error.json"), s.response.Body.String())
}

func (s *MiddlewareCommonSuite) Test_EntityUUIDFromRequestContext_Error() {
	// given
	ctx := context.Background()

	// when
	entityUUID, err := turtleware.EntityUUIDFromRequestContext(ctx)

	// then
	s.Empty(entityUUID)
	s.ErrorIs(err, turtleware.ErrContextMissingEntityUUID)
}

func (s *MiddlewareCommonSuite) Test_AuthBearerHeaderMiddleware_Success() {
	// given
	recordedToken := ""
	middlewareVerify := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		token, err := turtleware.AuthTokenFromRequestContext(r.Context())
		s.Require().NoError(err)

		recordedToken = token
	})

	middleware := turtleware.AuthBearerHeaderMiddleware

	s.request.Header.Set("Authorization", "Bearer 123")

	// when
	middleware(middlewareVerify).ServeHTTP(s.response, s.request)

	// then
	s.Equal("123", recordedToken)
	s.Empty(s.response.Body.String())
}

func (s *MiddlewareCommonSuite) Test_AuthBearerHeaderMiddleware_ErrMissingAuthHeader() {
	// given
	middlewareVerify := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		s.Fail("unexpected middleware invocation")
	})

	middleware := turtleware.AuthBearerHeaderMiddleware

	s.request.Header.Set("Authorization", "")

	// when
	middleware(middlewareVerify).ServeHTTP(s.response, s.request)

	// then
	s.Equal("bearer", s.response.Header().Get("WWW-Authenticate"))
	s.Equal(http.StatusUnauthorized, s.response.Code)
	s.JSONEq(s.loadTestDataString("authbearerheader/missing_auth_header.json"), s.response.Body.String())
}

func (s *MiddlewareCommonSuite) Test_AuthBearerHeaderMiddleware_ErrAuthHeaderWrongFormat() {
	// given
	middlewareVerify := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		s.Fail("unexpected middleware invocation")
	})

	middleware := turtleware.AuthBearerHeaderMiddleware

	s.request.Header.Set("Authorization", "borked")

	// when
	middleware(middlewareVerify).ServeHTTP(s.response, s.request)

	// then
	s.Equal(http.StatusBadRequest, s.response.Code)
	s.JSONEq(s.loadTestDataString("authbearerheader/wrong_auth_header_format.json"), s.response.Body.String())
}

func (s *MiddlewareCommonSuite) Test_AuthTokenFromRequestContext_Error() {
	// given
	ctx := context.Background()

	// when
	token, err := turtleware.AuthTokenFromRequestContext(ctx)

	// then
	s.Empty(token)
	s.ErrorIs(err, turtleware.ErrContextMissingAuthToken)
}

func (s *MiddlewareCommonSuite) Test_PagingMiddleware_Success() {
	// given
	recordedPaging := turtleware.Paging{}
	middlewareVerify := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		paging, err := turtleware.PagingFromRequestContext(r.Context())
		s.Require().NoError(err)

		recordedPaging = paging
	})

	middleware := turtleware.PagingMiddleware

	// when
	middleware(middlewareVerify).ServeHTTP(s.response, s.request)

	// then
	s.Equal(turtleware.Paging{
		Offset: 0,
		Limit:  100,
	}, recordedPaging)
	s.Empty(s.response.Body.String())
}

func (s *MiddlewareCommonSuite) Test_PagingMiddleware_ErrInvalidOffset() {
	// given
	recordedPaging := turtleware.Paging{}
	middlewareVerify := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		paging, err := turtleware.PagingFromRequestContext(r.Context())
		s.Require().NoError(err)

		recordedPaging = paging
	})

	middleware := turtleware.PagingMiddleware

	s.request.URL.RawQuery = "offset=kaese"

	// when
	middleware(middlewareVerify).ServeHTTP(s.response, s.request)

	// then
	s.Equal(turtleware.Paging{
		Offset: 0,
		Limit:  0,
	}, recordedPaging)
	s.Equal(http.StatusInternalServerError, s.response.Code)
	s.JSONEq(s.loadTestDataString("paging/invalid_offset.json"), s.response.Body.String())
}

func (s *MiddlewareCommonSuite) Test_AuthClaimsMiddleware_Success() {
	// given
	recordedClaims := map[string]interface{}{}
	recordedUserUUID := uuid.Nil
	middlewareVerify := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		claims, err := turtleware.AuthClaimsFromRequestContext(r.Context())
		s.Require().NoError(err)
		recordedClaims = claims

		userUUID, err := turtleware.UserUUIDFromRequestContext(r.Context())
		s.Require().NoError(err)
		recordedUserUUID = userUUID
	})

	// when
	// Contains AuthClaimsMiddleware for testing
	s.buildAuthChain(middlewareVerify).ServeHTTP(s.response, s.request)

	// then
	s.Equal(map[string]interface{}{
		"uuid": s.userUUID.String(),
	}, recordedClaims)
	s.Equal(s.userUUID, recordedUserUUID)
}

func (s *MiddlewareCommonSuite) Test_AuthClaimsMiddleware_ErrContextMissingAuthToken() {
	// given
	middlewareVerify := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		s.Fail("unexpected middleware invocation")
	})

	// when
	turtleware.AuthClaimsMiddleware(nil)(middlewareVerify).ServeHTTP(s.response, s.request)

	// then
	s.Equal(http.StatusInternalServerError, s.response.Code)
	s.JSONEq(s.loadTestDataString("authclaims/context_missing.json"), s.response.Body.String())
}

func (s *MiddlewareCommonSuite) Test_AuthClaimsMiddleware_ErrTokenValidationFailed() {
	// given
	middlewareVerify := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		s.Fail("unexpected middleware invocation")
	})

	s.request.Header.Set("Authorization", "Bearer 123")

	// when
	alice.New(
		turtleware.AuthBearerHeaderMiddleware,
		turtleware.AuthClaimsMiddleware(nil),
	).Then(middlewareVerify).ServeHTTP(s.response, s.request)

	// then
	s.Equal(http.StatusBadRequest, s.response.Code)
	s.JSONEq(s.loadTestDataString("authclaims/token_validation_failed.json"), s.response.Body.String())
}

func (s *MiddlewareCommonSuite) Test_PagingFromRequestContext_Error() {
	// given
	ctx := context.Background()

	// when
	paging, err := turtleware.PagingFromRequestContext(ctx)

	// then
	s.Equal(turtleware.Paging{
		Offset: 0,
		Limit:  0,
	}, paging)
	s.ErrorIs(err, turtleware.ErrContextMissingPaging)
}

func (s *MiddlewareCommonSuite) Test_AuthClaimsFromRequestContext_Error() {
	// given
	ctx := context.Background()

	// when
	claims, err := turtleware.AuthClaimsFromRequestContext(ctx)

	// then
	s.Nil(claims)
	s.ErrorIs(err, turtleware.ErrContextMissingAuthClaims)
}

func (s *MiddlewareCommonSuite) Test_UserUUIDFromRequestContext_Error() {
	// given
	ctx := context.Background()

	// when
	userUUID, err := turtleware.UserUUIDFromRequestContext(ctx)

	// then
	s.Empty(userUUID)
	s.ErrorIs(err, turtleware.ErrContextMissingAuthClaims)
}

func (s *MiddlewareCommonSuite) Test_UserUUIDFromRequestContext_ErrMissingUserUUID() {
	// given
	s.userUUID = uuid.Nil

	var capturedErr error
	chain := s.buildAuthChain(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, capturedErr = turtleware.UserUUIDFromRequestContext(r.Context())
	}))

	// when
	chain.ServeHTTP(s.response, s.request)

	// then
	s.ErrorIs(capturedErr, turtleware.ErrMissingUserUUID)
}
