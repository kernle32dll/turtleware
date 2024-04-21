package tenant_test

import (
	"context"
	"github.com/kernle32dll/turtleware/tenant"
	"github.com/stretchr/testify/suite"
	"strings"

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

func (s *MiddlewareCommonSuite) Test_UUIDMiddleware_Success() {
	// given
	recordedUUID := ""
	middlewareVerify := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		tenantUUID, err := tenant.UUIDFromRequestContext(r.Context())
		s.Require().NoError(err)

		recordedUUID = tenantUUID
	})

	// when
	s.buildAuthChain(middlewareVerify).ServeHTTP(s.response, s.request)

	// then
	s.Empty(s.response.Body.String())
	s.Equal(s.tenantUUID, recordedUUID)
}

func (s *MiddlewareCommonSuite) Test_UUIDMiddleware_Error() {
	// given
	middlewareVerify := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		s.Fail("unexpected middleware invocation")
	})

	middleware := tenant.UUIDMiddleware

	// when
	middleware(middlewareVerify).ServeHTTP(s.response, s.request)

	// then
	s.Equal(http.StatusInternalServerError, s.response.Code)
	s.JSONEq(s.loadTestDataString("errors/missing_auth_claims_error.json"), s.response.Body.String())
}

func (s *MiddlewareCommonSuite) Test_UUIDFromRequestContext_Error() {
	// given
	ctx := context.Background()

	// when
	tenantUUID, err := tenant.UUIDFromRequestContext(ctx)

	// then
	s.Empty(tenantUUID)
	s.ErrorIs(err, tenant.ErrContextMissingTenantUUID)
}

func (s *MiddlewareCommonSuite) Test_UUIDFromRequestContext_ErrTokenMissingTenantUUID() {
	// given
	s.tenantUUID = ""

	chain := s.buildAuthChain(nil)

	// when
	chain.ServeHTTP(s.response, s.request)

	// then
	s.True(strings.Contains(s.response.Body.String(), tenant.ErrTokenMissingTenantUUID.Error()))
}
