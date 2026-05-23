package ingest

import (
	"testing"

	"github.com/bolke/inv-driver/codec"
	"github.com/bolke/inv-driver/wire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// protoPublishesAValue reports whether readingToProto set any optional
// protection field (i.e. a field other than the always-present header
// peer_uid/ts_ms/model) on the message.
func protoPublishesAValue(p *wire.Protection) bool {
	published := false
	p.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		switch fd.Name() {
		case "peer_uid", "ts_ms", "model":
			return true // header fields — keep scanning
		default:
			published = true
			return false // found a real field; stop
		}
	})
	return published
}

// TestReadingToProto_NoSilentDrop guards the publish boundary: every code the
// codec can decode must be EITHER mapped onto wire.Protection by readingToProto
// OR explicitly registered in protCodesUnpublished. A code that is neither is a
// silent drop — a decoded protection value that vanishes before SunSpec with no
// trace — which this test fails on. It also flags stale unpublished entries and
// double-classified codes.
func TestReadingToProto_NoSilentDrop(t *testing.T) {
	all := codec.AllProtectionCodes()
	if len(all) == 0 {
		t.Fatal("codec.AllProtectionCodes() returned nothing")
	}
	decodable := make(map[string]bool, len(all))
	for _, code := range all {
		decodable[code] = true
		r := &codec.ProtectionReading{Values: map[string]float64{code: 7}}
		published := protoPublishesAValue(readingToProto("aabbccddeeff", 1, r))
		_, unpublished := protCodesUnpublished[code]

		switch {
		case published && unpublished:
			t.Errorf("code %s is both published by readingToProto and listed in protCodesUnpublished — pick one", code)
		case !published && !unpublished:
			t.Errorf("code %s is decodable but neither published nor in protCodesUnpublished — silent drop; map it in readingToProto or document it there", code)
		}
	}
	// No stale entries: every documented-unpublished code must really be decodable.
	for code := range protCodesUnpublished {
		if !decodable[code] {
			t.Errorf("protCodesUnpublished lists %s but the codec never decodes it (stale entry)", code)
		}
	}
}
