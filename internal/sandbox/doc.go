// Package sandbox defines the agent execution boundary. CLI reviews use Docker
// isolation; TrustedHostLauncher is retained only for offline protocol tests.
// TestRunner executes approved test profiles in separate disposable containers.
package sandbox
