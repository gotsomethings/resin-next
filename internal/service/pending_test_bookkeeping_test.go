package service

import (
	"testing"
	"time"

	"github.com/Resinat/Resin/internal/node"
)

// A node must stop being 待测 as soon as ANY probe path has run. Regression:
// keying only on LastProbeAttempt (written by the platform-level probe) left
// nodes probed by the node-level latency loop permanently pending, which also
// hid their reference latency in the node list.
func TestNodeIsPendingTest_AcceptsEitherProbePath(t *testing.T) {
	entry := &node.NodeEntry{}
	if !nodeIsPendingTest(entry) {
		t.Fatal("a fresh entry must be pending")
	}

	entry.LastLatencyProbeAttempt.Store(time.Now().UnixNano())
	if nodeIsPendingTest(entry) {
		t.Fatal("a node-level latency probe attempt must clear pending")
	}

	platformProbed := &node.NodeEntry{}
	platformProbed.LastProbeAttempt.Store(time.Now().UnixNano())
	if nodeIsPendingTest(platformProbed) {
		t.Fatal("a platform probe attempt must clear pending")
	}
}
