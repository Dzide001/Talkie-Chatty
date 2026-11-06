# APNs PTT transport

New selects only Apple's sandbox or production host, uses verified TLS with HTTP/2 support, disables redirects, and bounds requests with a five-second timeout. Send emits a minimal PTT payload with topic suffix .voip-ptt, push type pushtotalk, priority 10 and expiry 0.

TokenSource supplies a current Apple provider JWT from a future signer/secret integration. The transport does not generate a JWT, load private keys, register a device, or prove token authenticity locally. A real provider signer and credentials are still required. No runtime feature calls this package yet.

Result.Accepted means an HTTP 200 response, not delivery. Non-success results distinguish invalid device registration and retryable provider failures. The coordinator must compare registration generations before invalidation and check the active burst/deadline before any retry. Retries are not automatic. Transport/credential errors redact the underlying error because it may contain a token-bearing URL or key details.

Tests inject an HTTP RoundTripper and simulate provider responses. They verify request construction, bounds, error classification and redaction. They do NOT establish actual HTTP/2 negotiation, network TLS interoperability, valid Apple credentials, APNs delivery, or an iPhone wake.

References:
- https://developer.apple.com/documentation/pushtotalk/creating-a-push-to-talk-app
- https://developer.apple.com/documentation/usernotifications/sending-notification-requests-to-apns
