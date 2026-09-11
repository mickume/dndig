module github.com/mickume/dndig

go 1.26.5

// AgentKit declares the module path github.com/agentfox/agentkit-go but
// lives at github.com/agent-fox-dev/coder; see docs/adr/01.
replace github.com/agentfox/agentkit-go => github.com/agent-fox-dev/coder v0.0.0-20260911102337-52452aec8d2f

require github.com/agentfox/agentkit-go v0.0.0
