// DigiPanel Agent Client (core/agent_client.go)
// The panel process is unprivileged; every operation that needs root is sent to
// the agent over its Unix socket as a named action with a validated parameter
// map. There is no shell, no free-form command, and no fallback that silently
// pretends an action succeeded.
// Zero emojis, strictly production grade.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

var ErrAgentUnavailable = errors.New("privileged agent is not reachable on its socket")

// AgentClient talks to the DigiPanel agent daemon.
type AgentClient struct {
	SocketPath string
	Timeout    time.Duration
}

// NewAgentClient returns a client for the given socket path. An empty path
// yields a client that reports the agent as unavailable on every call.
func NewAgentClient(socketPath string) *AgentClient {
	return &AgentClient{SocketPath: socketPath, Timeout: 10 * time.Second}
}

// Available reports whether the client has a socket to talk to.
func (c *AgentClient) Available() bool {
	return c != nil && c.SocketPath != ""
}

type agentRequest struct {
	Action string            `json:"action"`
	Params map[string]string `json:"params"`
}

type agentResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Call sends one whitelisted action and returns its output.
func (c *AgentClient) Call(ctx context.Context, action string, params map[string]string) (string, error) {
	if !c.Available() {
		return "", ErrAgentUnavailable
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dialer := net.Dialer{}
	connection, err := dialer.DialContext(dialCtx, "unix", c.SocketPath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrAgentUnavailable, err)
	}
	defer connection.Close()

	deadline := time.Now().Add(timeout)
	_ = connection.SetDeadline(deadline)

	if err := json.NewEncoder(connection).Encode(agentRequest{Action: action, Params: params}); err != nil {
		return "", fmt.Errorf("send agent request: %w", err)
	}

	var response agentResponse
	if err := json.NewDecoder(connection).Decode(&response); err != nil {
		return "", fmt.Errorf("read agent response: %w", err)
	}
	if !response.Success {
		message := response.Error
		if message == "" {
			message = "agent rejected the action"
		}
		return "", errors.New(message)
	}
	return response.Output, nil
}
