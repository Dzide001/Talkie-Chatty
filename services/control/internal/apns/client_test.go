package apns

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixture() Wake {
	return Wake{"0.1-draft.2", "room", "epoch", "burst", "12345678-1234-4234-8234-123456789abc", "speaker"}
}
func testClient(t *testing.T, f transportFunc) *Client {
	t.Helper()
	c, e := New("sandbox", "org.example.talkie", func(context.Context) (string, error) { return "fixture.jwt.signature", nil })
	if e != nil {
		t.Fatal(e)
	}
	c.http.Transport = f
	return c
}
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, ProtoMajor: 2, Header: http.Header{"Apns-Id": []string{"request-id"}}, Body: io.NopCloser(strings.NewReader(body))}
}
func TestPTTRequest(t *testing.T) {
	c := testClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.sandbox.push.apple.com" || r.URL.Path != "/3/device/aabb" || r.Method != "POST" {
			t.Fatal("wrong endpoint")
		}
		for k, v := range map[string]string{"apns-push-type": "pushtotalk", "apns-topic": "org.example.talkie.voip-ptt", "apns-priority": "10", "apns-expiration": "0", "authorization": "bearer fixture.jwt.signature"} {
			if r.Header.Get(k) != v {
				t.Fatalf("wrong header %s", k)
			}
		}
		b, _ := io.ReadAll(r.Body)
		if len(b) > 4096 || !strings.Contains(string(b), `"burstId":"burst"`) || strings.Contains(string(b), "signature") {
			t.Fatal("wrong payload")
		}
		return response(200, ""), nil
	})
	r, e := c.Send(context.Background(), "aabb", fixture())
	if e != nil || !r.Accepted {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestProviderFailures(t *testing.T) {
	for _, tc := range []struct {
		status         int
		body           string
		retry, invalid bool
	}{{410, `{"reason":"Unregistered"}`, false, true}, {429, `{"reason":"TooManyRequests"}`, true, false}, {503, `{"reason":"ServiceUnavailable"}`, true, false}, {403, `{"reason":"InvalidProviderToken"}`, false, false}, {400, `{"reason":"attacker-controlled text"}`, false, false}} {
		c := testClient(t, func(*http.Request) (*http.Response, error) { return response(tc.status, tc.body), nil })
		r, e := c.Send(context.Background(), "aabb", fixture())
		if e != nil || r.Accepted || r.Retryable != tc.retry || r.InvalidateToken != tc.invalid {
			t.Fatalf("%+v %v", r, e)
		}
		if strings.Contains(r.Reason, "attacker") {
			t.Fatal("untrusted reason leaked")
		}
	}
}
func TestInvalidInputsDoNotSend(t *testing.T) {
	c := testClient(t, func(*http.Request) (*http.Response, error) { t.Fatal("sent invalid request"); return nil, nil })
	if _, e := c.Send(context.Background(), "../secret", fixture()); e == nil {
		t.Fatal("invalid token")
	}
	w := fixture()
	w.BurstID = ""
	if _, e := c.Send(context.Background(), "aabb", w); e == nil {
		t.Fatal("empty burst")
	}
	if _, e := New("other", "org.example", func(context.Context) (string, error) { return "", nil }); e == nil {
		t.Fatal("invalid environment")
	}
}
func TestTransportErrorsRedacted(t *testing.T) {
	c := testClient(t, func(*http.Request) (*http.Response, error) { return nil, errors.New("device-secret provider-secret") })
	_, e := c.Send(context.Background(), "aabb", fixture())
	if e == nil || strings.Contains(e.Error(), "secret") {
		t.Fatal(e)
	}
	c.token = func(context.Context) (string, error) { return "", errors.New("private signing key") }
	_, e = c.Send(context.Background(), "aabb", fixture())
	if e == nil || strings.Contains(e.Error(), "signing") {
		t.Fatal(e)
	}
}
func TestResponseLimitsAndProtocol(t *testing.T) {
	for _, tc := range []struct {
		body  string
		major int
	}{{strings.Repeat("x", 4097), 2}, {"", 1}} {
		c := testClient(t, func(*http.Request) (*http.Response, error) {
			r := response(200, tc.body)
			r.ProtoMajor = tc.major
			return r, nil
		})
		if _, e := c.Send(context.Background(), "aabb", fixture()); e == nil {
			t.Fatal("invalid response accepted")
		}
	}
}
