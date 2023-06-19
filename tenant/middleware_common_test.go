package tenant_test

import (
	"github.com/google/uuid"
	"github.com/kernle32dll/turtleware"
	"github.com/kernle32dll/turtleware/tenant"
	"github.com/stretchr/testify/suite"

	"context"
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

func (s *MiddlewareCommonSuite) Test_UUIDMiddleware_ErrContextMissingAuthClaims() {
	// given
	nextCapture := &MiddlewareCapture{}

	// when
	tenant.UUIDMiddleware()(nextCapture).ServeHTTP(s.response, s.request)

	// then
	s.Equal(http.StatusInternalServerError, s.response.Code)
	s.JSONEq(s.loadTestDataString("authclaims/context_missing.json"), s.response.Body.String())
	s.False(nextCapture.Called)
}

func (s *MiddlewareCommonSuite) Test_UUIDMiddleware_ErrTokenMissingTenantUUID() {
	// given
	s.tenantUUID = uuid.Nil
	nextCapture := &MiddlewareCapture{}

	// when
	// Contains tenant.UUIDMiddleware for testing
	s.buildAuthChain(nextCapture).ServeHTTP(s.response, s.request)

	// then
	s.Equal(http.StatusBadRequest, s.response.Code)
	s.JSONEq(s.loadTestDataString("authclaims/token_missing_uuid.json"), s.response.Body.String())
	s.False(nextCapture.Called)
}

func (s *MiddlewareCommonSuite) Test_UUIDMiddleware_Success() {
	// given
	recordedClaims := map[string]interface{}{}
	recordedTenantUUID := uuid.Nil
	middlewareVerify := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		claims, err := turtleware.AuthClaimsFromRequestContext(r.Context())
		s.Require().NoError(err)
		recordedClaims = claims

		tenantUUID, err := tenant.UUIDFromRequestContext(r.Context())
		s.Require().NoError(err)
		recordedTenantUUID = tenantUUID
	})

	// when
	// Contains tenant.UUIDMiddleware for testing
	s.buildAuthChain(middlewareVerify).ServeHTTP(s.response, s.request)

	// then
	s.Equal(map[string]interface{}{
		"uuid":        s.userUUID.String(),
		"tenant_uuid": s.tenantUUID.String(),
	}, recordedClaims)
	s.Equal(s.tenantUUID, recordedTenantUUID)
}

func (s *MiddlewareCommonSuite) Test_UUIDFromRequestContext_ErrContextMissingTenantUUID() {
	// given
	ctx := context.Background()

	// when
	tenantUUID, err := tenant.UUIDFromRequestContext(ctx)

	// then
	s.Empty(tenantUUID)
	s.ErrorIs(err, tenant.ErrContextMissingTenantUUID)
}
