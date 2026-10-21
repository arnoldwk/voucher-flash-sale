## Trunk-Based Development PR Checklist

### Type of Change
- [ ] 🚀 New Feature (behind feature flag if incomplete)
- [ ] 🐛 Bugfix
- [ ] ⚡ Performance Optimization
- [ ] 🧹 Refactoring / Clean Architecture update
- [ ] 📝 Documentation / Tests

### Trunk-Based Principles
- [ ] **Small & Atomic**: PR targets `main` directly and touches minimal files (< 400 lines ideally).
- [ ] **Fast Feedback**: All tests pass locally (`make test`) and in CI.
- [ ] **Zero Breaking Changes**: Database migrations are backward-compatible.
- [ ] **No Dead Code**: Experimental code is toggleable or cleanly isolated.

### Verification
- [ ] Unit tests added / updated.
- [ ] Race conditions tested (`go test -race ./...`).
- [ ] Manual test or load test executed.
