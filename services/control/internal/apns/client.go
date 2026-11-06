// Package apns provides the PTT provider transport. Accepted means Apple accepted
// a request, never that a device woke or delivered audio. Token signing and secure
// credential storage are injected; this package does not log credentials.
package apns

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type TokenSource func(context.Context) (string, error)
type Wake struct {
	ProtocolVersion string `json:"protocolVersion"`
	RoomID          string `json:"roomId"`
	RoomEpoch       string `json:"roomEpoch"`
	BurstID         string `json:"burstId"`
	ChannelID       string `json:"channelId"`
	SpeakerID       string `json:"speakerId"`
}
type Result struct {
	Accepted        bool
	Status          int
	RequestID       string
	Reason          string
	Retryable       bool
	InvalidateToken bool
}
type Client struct {
	endpoint, topic string
	token           TokenSource
	http            *http.Client
}

var bundlePattern = regexp.MustCompile(`^[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+$`)
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func New(environment, bundleID string, token TokenSource) (*Client, error) {
	endpoint := ""
	switch environment {
	case "sandbox":
		endpoint = "https://api.sandbox.push.apple.com"
	case "production":
		endpoint = "https://api.push.apple.com"
	default:
		return nil, errors.New("invalid APNs environment")
	}
	if token == nil || len(bundleID) > 240 || !bundlePattern.MatchString(bundleID) {
		return nil, errors.New("invalid APNs configuration")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ForceAttemptHTTP2 = true
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	return &Client{endpoint: endpoint, topic: bundleID + ".voip-ptt", token: token, http: &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) Send(ctx context.Context, deviceToken string, wake Wake) (Result, error) {
	if len(deviceToken) < 2 || len(deviceToken) > 1024 {
		return Result{}, errors.New("invalid APNs device token")
	}
	if _, err := hex.DecodeString(deviceToken); err != nil {
		return Result{}, errors.New("invalid APNs device token")
	}
	for _, s := range []string{wake.RoomID, wake.RoomEpoch, wake.BurstID, wake.SpeakerID} {
		if len(s) == 0 || len(s) > 128 {
			return Result{}, errors.New("invalid wake metadata")
		}
	}
	if wake.ProtocolVersion != "0.1-draft.2" || !uuidPattern.MatchString(wake.ChannelID) {
		return Result{}, errors.New("invalid wake version or channel")
	}
	// No SDP, voice data, media keys or arbitrary caller-supplied payload fields.
	payload, err := json.Marshal(struct {
		Wake
		APS map[string]int `json:"aps"`
	}{wake, map[string]int{"content-available": 1}})
	if err != nil || len(payload) > 4096 {
		return Result{}, errors.New("APNs payload exceeds bound")
	}
	jwt, err := c.token(ctx)
	if err != nil {
		return Result{}, errors.New("APNs provider credential unavailable")
	}
	if len(jwt) > 4096 || strings.Count(jwt, ".") != 2 || strings.ContainsAny(jwt, "\r\n ") {
		return Result{}, errors.New("invalid APNs provider credential")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/3/device/"+deviceToken, bytes.NewReader(payload))
	if err != nil {
		return Result{}, errors.New("APNs request construction failed")
	}
	req.Header.Set("authorization", "bearer "+jwt)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("apns-push-type", "pushtotalk")
	req.Header.Set("apns-topic", c.topic)
	req.Header.Set("apns-priority", "10")
	req.Header.Set("apns-expiration", "0")
	response, err := c.http.Do(req)
	if err != nil {
		return Result{}, errors.New("APNs transport failed")
	}
	defer response.Body.Close()
	if response.ProtoMajor != 2 {
		return Result{}, errors.New("APNs requires HTTP/2")
	}
	result := Result{Status: response.StatusCode, RequestID: response.Header.Get("apns-id")}
	if len(result.RequestID) > 128 {
		result.RequestID = ""
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil || len(body) > 4096 {
		return result, errors.New("invalid APNs response")
	}
	if response.StatusCode == http.StatusOK {
		result.Accepted = true
		return result, nil
	}
	var failure struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(body, &failure)
	switch failure.Reason {
	case "Unregistered", "BadDeviceToken", "DeviceTokenNotForTopic":
		result.Reason = failure.Reason
		result.InvalidateToken = response.StatusCode == 400 || response.StatusCode == 410
	case "TooManyRequests", "InternalServerError", "ServiceUnavailable", "ExpiredProviderToken", "InvalidProviderToken", "BadTopic", "TopicDisallowed", "PayloadTooLarge":
		result.Reason = failure.Reason
	default:
		result.Reason = "ProviderRejected"
	}
	result.Retryable = response.StatusCode == 429 || response.StatusCode >= 500
	// No automatic retry: the coordinator must first check the current burst and
	// deadline. Registration invalidation must compare the registration generation.
	return result, nil
}
