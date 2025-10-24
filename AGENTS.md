# Agent Instructions

This document contains coding standards, architectural patterns, and conventions for AI agents working on this codebase.

## Core Principles

### 1. Understand Before Implementing
- **Never start coding before understanding requirements fully**
- Ask clarifying questions if requirements are ambiguous
- Review existing patterns and follow them

### 2. YAGNI (You Aren't Gonna Need It) - Balanced Approach
- Only implement what's actually needed **now**
- **Balance:** Start with proper architecture that enables extension, but don't over-engineer
- Good abstractions make future development easier, but avoid adding unused features "just in case"
- Remove code that's not currently used, but design interfaces that can accommodate future needs
- **Rule of thumb:** Build for today's requirements with tomorrow's extensibility in mind, not next year's hypothetical features

### 3. Package Organization
- Each component should be self-contained
- Device-specific code stays with the device (e.g., `dishwasher/profile.go`)
- Generic utilities go in appropriate shared packages
- **Profile types belong to their domain:**
  - `scheduler.Profile` - for scheduled devices with stages (dishwasher, dryer)
  - `charger.ChargingProfile` - for continuous charging devices
  - `optimizer` package - pure optimization algorithms, no domain-specific types

### 4. Component Architecture
- All components embed `component.Base` for common services
- Components declare their own listener needs via methods:
  - `EventListeners()` - React to custom events
  - `EntityListeners()` - React to entity state changes  
  - `DailySchedules()` - Run at specific times daily
  - `Intervals()` - Run periodically
- Each component is registered in `main.go`

### State Persistence
- Use Home Assistant entities for state that must survive restarts
- Entity types available:
  - `input_boolean` - on/off states
  - `input_select` - mode selection
  - `input_datetime` - scheduled times
  - `input_number` - numeric values
  - `input_text` - text storage
- Ask the user to define new entities in Home Assistant if needed

## Code Quality Standards

### Architecture vs. Simplicity Balance
- **Good architecture enables growth** - Clean interfaces, proper separation of concerns, extensible patterns
- **Over-engineering hinders velocity** - Complex abstractions for hypothetical use cases, excessive indirection
- **When to invest in architecture:**
  - You see a clear pattern emerging (e.g., multiple devices need optimization)
  - The abstraction simplifies current code, not just future code
  - Cost of refactoring later is high (breaking changes, data migration)
- **When to stay simple:**
  - It's the first implementation of its kind
  - Requirements are unclear or likely to change drastically
  - The "reuse" scenario is purely hypothetical
- **Example:** We created unified `optimizer.Optimizer` because dishwasher and chargers both needed price optimization (real need), not because "maybe someday we'll optimize heating" (hypothetical)

### Testing
- Use table-driven tests for comprehensive coverage
- Mock external dependencies using `mockgen`
- Aim for meaningful coverage, not just high percentages
- Test edge cases and error paths
- **ALWAYS update tests when adding new features or modifying existing code**
- **Ensure test coverage for all new functions and logic paths**
- Run `make test` before considering work complete

### Error Handling
- Always handle errors explicitly
- Log errors with context
- Return errors up the stack, don't swallow them
- Use `fmt.Errorf()` for error wrapping

### Logging
- Use `debug.Log()` for verbose development logs
- Control via `DEBUG=true` environment variable
- Use standard `log.Printf()` for important events
- Include context in log messages

### Code Readability
- **Keep functions small and readable like a book** - each function should do one thing
- Extract complex logic into well-named helper methods
- Aim for functions under 20-30 lines when possible
- If a function has multiple responsibilities, split it into smaller, focused functions
- Example: Instead of one 100-line function with nested conditionals, create 4-5 functions with descriptive names that read naturally

### Cyclomatic Complexity
- Keep functions under complexity 15
- Extract helper functions when complexity grows
- Use early returns to reduce nesting

## Architecture Patterns

### Optimization Strategy

**Two distinct use cases:**

1. **Scheduled Optimization** (continuous block)
   - Used for: Dishwasher, dryer, washing machine
   - Method: `optimizer.Optimize(profile, priceSlots)`
   - Profile needs: `Duration`, `StageWeights`, `PowerKW`
   - Finds best **start time** for a fixed-duration cycle

2. **Continuous Optimization** (per time slot)
   - Used for: Laptop charger, vacuum charger, heating
   - Method: `optimizer.OptimizeCheapestHours(request, priceSlots)`
   - Request needs: `TotalDuration`, `WindowSize`
   - Finds **cheapest slots** within a window to charge/run

### Dynamic Savings Threshold
- Uses exponential decay: `threshold = base + scale * e^(-decay * hours)`
- More wait time allowed = lower threshold required
- Special case: Night mode accepts any positive savings
- No hard thresholds, smooth mathematical function

### Presence Detection
- `component.Base` provides house mode helpers:
  - `IsAway()` - Check if house mode is Away/Travel
  - `IsAwayForDuration(duration)` - Check prolonged absence
  - `IsNightMode()` - Check if daytime mode is Night
  - `GetHouseMode()` - Get current house mode
- Use for safety features (e.g., disable chargers when away >2h)

### Notifications
- Fire custom events via `notifications.NotificationService`
- Each device constructs the complete message in human-readable format
- Home Assistant automations handle text-to-speech delivery
- Event type: `event.custom_notify` (defined in `custom_entities.go`)
- Use `notifications.FormatTimeForSpeech()` for natural time formats (e.g., "3 PM", "noon")

### Dry-Run Mode
- Set `DRY_RUN=true` to test without actual device control
- Wrapper in `dryrun` package logs actions instead of executing
- Use for testing logic before deploying

## Naming Conventions

### Packages
- Device packages use singular names: `dishwasher`, `laptop`, not `dishwashers`
- Group related functionality: `scheduler/optimizer`, `charger/laptop`

### Files
- `component.go` - Main component implementation
- `profile.go` - Device profiles and modes
- `*_test.go` - Tests for the corresponding file

### Types
- Components: `Dishwasher`, `LaptopCharger` (not `DishwasherComponent`)
- Profiles: `Profile`, `ChargingProfile` (specific to domain)
- Interfaces: Clear purpose in name (`DeviceProfile`, `Component`)

## Time Handling

### Precision
- Use `time.Duration` for exact timing, not hours as `int`
- Example: `137 * time.Minute`, not `2` hours (rounded)
- Match optimization intervals to pricing granularity:
  - Pricing updates: 15 minutes
  - Charger optimization: 15 minutes (not 1 hour)

### Notifications
- Use `notifications.FormatTimeForSpeech()` for human-readable for time formatting
- Examples: "3 PM", "3:30 PM", "noon", "midnight"
- Avoid technical formats: "15:00", "03:00"
- General rule: if talking to a human, use human language

## Git Workflow

### Commit Approval
- **NEVER commit code without explicit user approval**
- **Wait for the user to say "commit" or "push" before executing git commands**
- Present a summary of changes and ask for approval first
- The user reviews and decides when code is ready to commit

### Commit Signing
- Different key for personal and work repos (configured in git config)
- All commits must be signed

### Branch Naming
- `feature/*` - New features
- `docs/*` - Documentation
- `fix/*` - Bug fixes
- `refactor/*` - Code restructuring

### Commit Messages
- Use conventional commits format
- `feat:` - New features
- `fix:` - Bug fixes
- `refactor:` - Code restructuring
- `docs:` - Documentation
- Include clear, concise description
- Multi-line for complex changes

### Home Assistant Integration

### Entity Naming
- Follow HA conventions: `domain.name`, specifically `domain.area_device_variable`, e.g. `input_text.kitchen_dishwasher_cost` means "an input text to store cost for dishwasher in the kitchen"
- Use underscores: `input_boolean.office_laptop_charge_optimization_auto`
- Define in `entities/custom_entities.go`. `entities/entities.go` is generated with `go generate`
- **NEVER use hardcoded entity strings in code** - always reference entities from `entities` package:
  - Auto-generated entities: `entities.InputBoolean.OfficeLaptopChargeOptimizationAuto`
  - Custom entities (events): `entities.CustomEvents.Notify`
  - Custom entities (sensors from external sources): `entities.CustomSensors.OfficeLaptopWorkInternalBatteryLevel`
- If you need a new entity, add it to `entities/custom_entities.go` first, then use the constant
- **IMPORTANT**: `entities/entities.go` is auto-generated from Home Assistant and contains ALL existing entities in the system
  - Only add to `custom_entities.go` if the entity is truly custom (events, helpers not in HA yet)
  - If entity exists in HA (like companion app sensors), it's already in `entities/entities.go` - use it directly

### Event-Driven Architecture
- Components react to events
- Use appropriate listener types
- Fire custom events for cross-component communication

### Automation Handoff
- Go service fires events with structured data
- Home Assistant automations handle presentation (TTS, mobile notifications, UI)
- Service constructs messages with full context in human-readable language
- Example: Dishwasher component constructs "Dishwasher starts at 3 PM, saving 7% on electricity", HA automation speaks it through appropriate speaker

## Performance Considerations

### Optimization Frequency
- Match pricing service update frequency
- Current: 15-minute intervals for pricing
- Chargers: 15-minute optimization cycles
- Scheduled devices: On-demand when triggered

### Resource Usage
- Avoid unnecessary API calls
- Cache when appropriate
- Use `debug.Log()` to avoid log spam in production

## Testing Philosophy

### What to Test
- Business logic and algorithms
- Error handling paths
- Edge cases (empty inputs, boundary values)
- Integration between components

### What Not to Over-Test
- Simple getters/setters
- Pass-through functions
- External library behavior

### Mock Strategy
- Use `mockgen` with real production interfaces
- Don't create test-only interfaces
- Keep mocks in `mocks/` directory
- Regenerate with `make mocks`

## Common Patterns

### Service Initialization
```go
// In main.go
base := component.NewBase(component.BaseConfig{
    Service: service,
    State:   app.GetState(),
})

comp := device.New(base, pricingService, ...)
```

### Profile Definition
```go
// In device/profile.go
var ProfileName = Profile{
    Mode:     "mode_name",
    Duration: 137 * time.Minute, // Exact, measured duration
    StageWeights: []float64{...},
    PowerKW:  2.0,
}
```

### Event Handling
```go
// In component.go
func (c *Component) EventListeners() []ga.EventListener {
    return []ga.EventListener{
        ga.NewEventListener().
            EventType(entities.CustomEvents.MyEvent).
            Handle(c.handleEvent).
            Build(),
    }
}
```

## Debugging

### Debug Mode
- Set `DEBUG=true` in environment
- Enables verbose logging via `debug.Log()`
- Shows optimization decisions, API calls, etc.

### Dry-Run Mode  
- Set `DRY_RUN=true` in environment
- Logs device control actions without executing
- Safe for testing logic

### VS Code Launch Config
- `.vscode/launch.json` configured for debugging
- Can set environment variables in launch config
- Breakpoints work normally

## Documentation

### Code Comments
- Document **why**, not what
- Explain business logic and decisions
- Use godoc format for public APIs
- Keep comments up to date with code

### README
- High-level architecture overview
- Setup and installation instructions
- Environment variables
- How to add new components
- Keep the list of available automations up-to-date

### This File (AGENTS.md)
- Patterns and conventions
- Architectural decisions
- Common pitfalls and how to avoid them
- Update when new patterns emerge

---

**Remember:** These are guidelines based on real development sessions. When in doubt, check existing code for patterns, and ask before implementing major changes.

