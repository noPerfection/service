package manager

import (
	"testing"

	"github.com/noPerfection/datatype"
	"github.com/noPerfection/protocol/message"
	"github.com/stretchr/testify/require"
)

func TestSecureOutboundItemsRoundTrip(t *testing.T) {
	routeA := "svc-a.main.one"
	routeB := "svc-a.main.two"
	items := datatype.New().
		Set(routeA, datatype.New().
			Set("public-key", "pub-a").
			Set("hmac", "hmac-a").
			Set("source-url", routeA).
			Set("target-url", "svc-root.main.one")).
		Set(routeB, datatype.New().
			Set("public-key", "pub-b").
			Set("hmac", "hmac-b").
			Set("source-url", routeB).
			Set("target-url", "svc-root.main.two"))

	req := &message.Request{
		Command: SecureOutbounds,
		Parameters: datatype.New().
			Set("context-url", "ctx.service-manager").
			Set("service-url", "root.service-manager").
			Set("outbounds", items),
	}

	packer := &message.MessagePacker{}
	envelope, err := packer.SerializeRequest(req)
	require.NoError(t, err)
	decoded, _, err := packer.DeserializeRequest(envelope)
	require.NoError(t, err)

	got, err := getSecureOutboundItems(decoded)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, outboundSecureItem{
		PublicKey: "pub-a",
		HMAC:      "hmac-a",
		SourceURL: routeA,
		TargetURL: "svc-root.main.one",
	}, got[routeA])
	require.Equal(t, outboundSecureItem{
		PublicKey: "pub-b",
		HMAC:      "hmac-b",
		SourceURL: routeB,
		TargetURL: "svc-root.main.two",
	}, got[routeB])
}

func TestSecureOutboundReplyKeysRoundTrip(t *testing.T) {
	req := &message.Request{Command: SecureOutbounds, Parameters: datatype.New()}
	reply := req.Ok(datatype.New().Set("outbound-route-public-keys", datatype.New().
		Set("svc-a.main.one", "route-pub-a").
		Set("svc-a.main.two", "route-pub-b")))

	packer := &message.MessagePacker{}
	envelope, err := packer.SerializeReply(reply)
	require.NoError(t, err)
	decoded, _, err := packer.DeserializeReply(envelope)
	require.NoError(t, err)
	require.True(t, decoded.IsOK())

	rawKeys, err := decoded.ReplyParameters().NestedValue("outbound-route-public-keys")
	require.NoError(t, err)
	keys := map[string]string{}
	require.NoError(t, rawKeys.Interface(&keys))
	require.Equal(t, map[string]string{
		"svc-a.main.one": "route-pub-a",
		"svc-a.main.two": "route-pub-b",
	}, keys)
}
