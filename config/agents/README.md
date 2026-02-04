# Agent Profiles

Each agent has four profile files generated from persona settings.

- `IDENTITY.md`: Stable identity facts (name, role, domains, traits)
- `SOUL.md`: Values, mission, and innovation scope
- `HEARTBEAT.md`: When to speak, when to stay silent, and rest protocol
- `USER.md`: Who the agent serves and collaboration style

Regenerate with:
```
go run ./cmd/gen_agent_profiles -out ./config/agents
```
