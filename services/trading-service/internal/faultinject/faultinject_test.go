package faultinject

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestEnabledAndGuardStartup(t *testing.T) {
	t.Setenv(envToggle, "")
	if Enabled() {
		t.Fatal("expected fault injection to be disabled")
	}

	t.Setenv(envToggle, " enabled ")
	if !Enabled() {
		t.Fatal("expected fault injection to be enabled")
	}

	gin.SetMode(gin.TestMode)
	if err := GuardStartup(); err != nil {
		t.Fatalf("GuardStartup in test mode returned error: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(gin.TestMode) })
	if err := GuardStartup(); err == nil {
		t.Fatal("expected GuardStartup to reject release mode with injection enabled")
	}
}

func TestFromHeadersParsesValidSpec(t *testing.T) {
	headers := http.Header{}
	headers.Set(HeaderForceFail, " f2 ")
	headers.Set(HeaderForceFailKind, "after")
	headers.Set(HeaderCompensateFail, " c3 ")
	headers.Set(HeaderCompensateFailTimes, "2")
	headers.Set(HeaderInjectDelay, " f4:150ms ")

	spec, err := FromHeaders(headers)
	if err != nil {
		t.Fatalf("FromHeaders returned error: %v", err)
	}
	if spec.ForceFailStep != "F2" || spec.ForceFailKind != KindAfter {
		t.Fatalf("unexpected force-fail spec %#v", spec)
	}
	if spec.CompensateFailStep != "C3" || spec.CompensateFailRemaining != 2 {
		t.Fatalf("unexpected compensate spec %#v", spec)
	}
	if spec.DelayStep != "F4" || spec.DelayMs != 150 {
		t.Fatalf("unexpected delay spec %#v", spec)
	}
}

func TestFromHeadersReturnsNilWhenNoFaultHeadersPresent(t *testing.T) {
	spec, err := FromHeaders(http.Header{"Other": []string{"value"}})
	if err != nil {
		t.Fatalf("FromHeaders returned error: %v", err)
	}
	if spec != nil {
		t.Fatalf("spec = %#v, want nil", spec)
	}
}

func TestFromHeadersRejectsMalformedHeaders(t *testing.T) {
	cases := []struct {
		name    string
		headers http.Header
	}{
		{name: "unknown forward step", headers: http.Header{HeaderForceFail: []string{"F9"}}},
		{name: "kind without forward step", headers: http.Header{HeaderForceFailKind: []string{KindAfter}}},
		{name: "unknown kind", headers: http.Header{HeaderForceFail: []string{"F1"}, HeaderForceFailKind: []string{"during"}}},
		{name: "unknown compensation step", headers: http.Header{HeaderCompensateFail: []string{"C9"}}},
		{name: "times without compensation step", headers: http.Header{HeaderCompensateFailTimes: []string{"2"}}},
		{name: "negative compensation times", headers: http.Header{HeaderCompensateFail: []string{"C1"}, HeaderCompensateFailTimes: []string{"-1"}}},
		{name: "invalid delay format", headers: http.Header{HeaderInjectDelay: []string{"F1"}}},
		{name: "invalid delay step", headers: http.Header{HeaderInjectDelay: []string{"C1:10ms"}}},
		{name: "invalid delay amount", headers: http.Header{HeaderInjectDelay: []string{"F1:slow"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := FromHeaders(tc.headers); err == nil {
				t.Fatal("expected parse error")
			}
		})
	}
}

func TestSpecMarshalUnmarshalAndContext(t *testing.T) {
	spec := &Spec{
		ForceFailStep:           "F1",
		ForceFailKind:           KindBefore,
		CompensateFailStep:      "C2",
		CompensateFailRemaining: 3,
		DelayStep:               "F5",
		DelayMs:                 250,
	}

	raw := spec.Marshal()
	if raw == "" {
		t.Fatal("expected marshaled spec")
	}
	roundTrip := Unmarshal(raw)
	if roundTrip == nil || roundTrip.ForceFailStep != "F1" || roundTrip.CompensateFailRemaining != 3 || roundTrip.DelayMs != 250 {
		t.Fatalf("unexpected unmarshaled spec %#v", roundTrip)
	}
	if (*Spec)(nil).Marshal() != "" {
		t.Fatal("nil spec should marshal to empty string")
	}
	if Unmarshal("") != nil {
		t.Fatal("empty string should unmarshal to nil")
	}
	if Unmarshal("{bad json") != nil {
		t.Fatal("invalid JSON should unmarshal to nil")
	}

	ctx := WithSpec(context.Background(), spec)
	if got := SpecFromContext(ctx); got != spec {
		t.Fatalf("SpecFromContext returned %#v", got)
	}
	if got := SpecFromContext(context.Background()); got != nil {
		t.Fatalf("unexpected spec on empty context %#v", got)
	}
}

func TestMiddlewareRejectsDisabledFaultHeadersAndAttachesEnabledSpec(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(envToggle, "")

	router := gin.New()
	router.Use(Middleware())
	router.GET("/exercise", func(c *gin.Context) {
		if SpecFromContext(c.Request.Context()) == nil {
			c.String(http.StatusOK, "no-spec")
			return
		}
		c.String(http.StatusOK, "has-spec")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/exercise", nil)
	req.Header.Set(HeaderForceFail, "F1")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("disabled middleware status = %d body=%s", rec.Code, rec.Body.String())
	}

	t.Setenv(envToggle, "true")
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/exercise", nil)
	req.Header.Set(HeaderForceFail, "F1")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "has-spec" {
		t.Fatalf("enabled middleware status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/exercise", nil)
	req.Header.Set(HeaderForceFail, "bad")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed middleware status = %d body=%s", rec.Code, rec.Body.String())
	}
}
