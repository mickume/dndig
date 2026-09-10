module github.com/mickume/dndig

go 1.26.5

// AgentKit declares the module path github.com/agentfox/agentkit-go but
// lives at github.com/agent-fox-dev/coder; see docs/adr/01.
replace github.com/agentfox/agentkit-go => github.com/agent-fox-dev/coder v0.0.0-20260910144839-da5b772f5a78

require github.com/agentfox/agentkit-go v0.0.0
