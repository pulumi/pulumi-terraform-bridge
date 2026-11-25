# Field-Level Conditional DeleteBeforeReplace Implementation Plan

## Executive Summary

This document outlines the implementation plan for adding field-level conditional `DeleteBeforeReplace` functionality to the Pulumi Terraform Bridge. The solution uses a predicate-based system to enable dynamic decision-making about when resource replacement should use delete-before-replace vs create-before-replace strategies.

**Key Innovation**: Instead of static boolean values, this implements a "truthiness function" approach using structured predicates that can evaluate resource state at runtime to make intelligent replacement decisions.

## Problem Statement

### Core Issue
Several bridged Terraform providers have fields that trigger resource replacement, but the replacement fails unless the resource is deleted before creating the replacement. Currently, `DeleteBeforeReplace` only works at the resource level, creating an all-or-nothing approach.

### Specific Examples

1. **Google Private EKS Clusters**: Field `enable_private_nodes` triggers replacement, but fails if attached network interfaces conflict between old and new resources
2. **Linode Instances**: Changes to `root_pass` on certain instance types require delete-before-replace to avoid configuration conflicts
3. **General Pattern**: The need for delete-before-replace is often conditional based on resource state, not just field changes

### Current Limitations
- Resource-level `DeleteBeforeReplace` is too broad (affects all field changes)
- No way to specify conditional logic (e.g., "only if attached resources exist")
- Users must manually configure `deleteBeforeReplace: true` with limited guidance

## Solution Architecture

### High-Level Approach

1. **Predicate-Based System**: Use structured predicates instead of boolean flags
2. **Runtime Evaluation**: Evaluate conditions during diff computation using actual resource state
3. **Warning-First Strategy**: Start with warnings to users, then add experimental automatic behavior
4. **Extensible Framework**: Allow new predicates to be added without breaking changes

### Core Components

#### 1. Schema Enhancement

```go
// Location: pkg/tfbridge/info/info.go
type Schema struct {
    // ... existing fields
    ForceNew                *bool
    DeleteBeforeReplace     *bool                           // Simple boolean case (unchanged)
    DeleteBeforeReplaceFunc *DeleteBeforeReplaceEvaluator  // NEW: Predicate-based evaluation
}

type DeleteBeforeReplaceEvaluator struct {
    // Structured predicates for transparency and debuggability
    Predicates []DeleteBeforeReplacePredicate
    Logic      PredicateLogic  // How to combine predicates (AND, OR, NOT)
    
    // Behavior control
    Behavior   DeleteBeforeReplaceBehavior  // Warn vs Auto-apply
    Reasoning  string                       // Human-readable explanation
}

type DeleteBeforeReplacePredicate struct {
    Type        PredicateType
    Config      map[string]interface{}
    Description string  // Human-readable description for debugging
}

type PredicateLogic struct {
    Op   string  // "AND", "OR", "NOT"
    Expr string  // Complex expressions like "(A AND B) OR C"
}

type DeleteBeforeReplaceBehavior int
const (
    WarnUser  DeleteBeforeReplaceBehavior = iota  // Default: warn user
    AutoApply                                     // Experimental: auto-apply
)
```

#### 2. Predicate Types

```go
type PredicateType string

// Core predicates for initial implementation
const (
    HasAttachedResources PredicateType = "hasAttachedResources"  // Check for dependent resources
    FieldChanged         PredicateType = "fieldChanged"         // Specific field has changed
    FieldEquals          PredicateType = "fieldEquals"          // Field equals specific value
    FieldMatches         PredicateType = "fieldMatches"         // Field matches regex/pattern
    ResourceCount        PredicateType = "resourceCount"        // Count of related resources
    StateCheck           PredicateType = "stateCheck"           // Custom state validation
)
```

#### 3. Runtime Evaluation Engine

```go
// Location: pkg/tfbridge/predicate_evaluator.go (NEW FILE)
type PredicateEvaluator struct {
    // Core evaluation logic
}

func (pe *PredicateEvaluator) Evaluate(
    predicates []DeleteBeforeReplacePredicate,
    logic PredicateLogic,
    old, new resource.PropertyMap,
    meta ResourceMetadata,
) (*EvaluationResult, error) {
    // Evaluate each predicate and combine using logic
}

type ResourceMetadata struct {
    ProviderName    string
    ResourceType    string
    AttachedResources []AttachedResourceInfo
    // Additional context as needed
}

type EvaluationResult struct {
    ShouldDeleteBeforeReplace bool
    Reasoning                 []string  // Step-by-step reasoning
    MatchedPredicates        []string  // Which predicates matched
}
```

## Implementation Plan

### Phase 1: Foundation Infrastructure (3 weeks)

#### Week 1: Core Schema and Predicate Framework
**Deliverables:**
- Add `DeleteBeforeReplaceEvaluator` to `pkg/tfbridge/info/info.go`
- Implement predicate type definitions
- Create `PredicateEvaluator` with basic evaluation logic
- Unit tests for predicate evaluation

**Files to Modify:**
- `pkg/tfbridge/info/info.go` - Add new schema structs
- `pkg/tfbridge/predicate_evaluator.go` - NEW: Core evaluation engine
- `pkg/tfbridge/predicate_evaluator_test.go` - NEW: Unit tests

**Risk Level:** Low - Pure additive changes, no behavior modification

#### Week 2: Core Predicate Implementations
**Deliverables:**
- Implement `HasAttachedResources` predicate
- Implement `FieldChanged` predicate  
- Implement `FieldEquals` predicate
- Comprehensive test suite for each predicate

**Files to Modify:**
- `pkg/tfbridge/predicates/` - NEW DIRECTORY: Individual predicate implementations
- `pkg/tfbridge/predicates/attached_resources.go` - NEW
- `pkg/tfbridge/predicates/field_operations.go` - NEW
- Corresponding test files

**Risk Level:** Low - Self-contained predicate logic

#### Week 3: Integration with Diff Logic
**Deliverables:**
- Integrate predicate evaluation into existing diff computation
- Add warning infrastructure for recommendations
- Preserve all existing behavior (no automatic application yet)

**Files to Modify:**
- `pkg/tfbridge/detailed_diff.go` - Add predicate evaluation
- `pkg/tfbridge/provider.go` - Integrate warnings into diff response
- `pkg/tfbridge/property_path.go` - Add evaluation utilities

**Risk Level:** Medium - Touches core diff logic, but only adds warnings

### Phase 2: Provider Integration & Testing (2 weeks)

#### Week 4: GCP Provider Integration
**Deliverables:**
- Implement GCP-specific predicate configurations
- Add `HasAttachedResources` detection for compute instances
- Comprehensive testing with real GCP scenarios

**Example Configuration:**
```go
// In GCP provider code
"enable_private_nodes": {
    DeleteBeforeReplaceFunc: &DeleteBeforeReplaceEvaluator{
        Predicates: []DeleteBeforeReplacePredicate{
            {
                Type: HasAttachedResources,
                Config: map[string]interface{}{
                    "resourceTypes": []string{"networkInterface", "attachedDisk"},
                    "checkConflicts": true,
                },
                Description: "Check for network interfaces that cannot be shared",
            },
            {
                Type: FieldChanged,
                Config: map[string]interface{}{
                    "field": "enable_private_nodes",
                },
                Description: "Private nodes setting is changing",
            },
        },
        Logic: PredicateLogic{Op: "AND"},
        Behavior: WarnUser,
        Reasoning: "Private node changes may conflict with attached network interfaces",
    },
}
```

**Files to Modify:**
- Provider-specific configuration files
- Integration tests with real GCP resources

**Risk Level:** Medium - Provider-specific changes, extensive testing required

#### Week 5: Linode Provider Integration & Cross-Framework Support
**Deliverables:**
- Implement Linode-specific configurations
- Ensure Plugin Framework support mirrors SDKv2
- Cross-provider testing and validation

**Example Configuration:**
```go
// In Linode provider code
"root_pass": {
    DeleteBeforeReplaceFunc: &DeleteBeforeReplaceEvaluator{
        Predicates: []DeleteBeforeReplacePredicate{
            {
                Type: FieldEquals,
                Config: map[string]interface{}{
                    "field": "type",
                    "values": []string{"g6-standard-1", "g6-standard-2"},
                },
                Description: "Instance type requires special handling",
            },
        },
        Logic: PredicateLogic{Op: "OR"},
        Behavior: WarnUser,
        Reasoning: "Root password changes on these instance types require delete-before-replace",
    },
}
```

**Files to Modify:**
- `pkg/pf/` - Plugin Framework equivalent implementations
- Provider-specific configurations
- Cross-framework test suite

**Risk Level:** Medium - Multi-framework coordination required

### Phase 3: Experimental Automatic Behavior (2 weeks)

#### Week 6: Feature Flag Implementation
**Deliverables:**
- Add `PULUMI_EXPERIMENTAL_AUTO_DELETE_BEFORE_REPLACE` feature flag
- Implement automatic application of delete-before-replace when flag is enabled
- Extensive logging and user feedback mechanisms

**Files to Modify:**
- `pkg/tfbridge/provider.go` - Add feature flag detection and automatic behavior
- Environment variable handling
- Logging infrastructure

**Risk Level:** High - Changes actual replacement behavior, requires extensive testing

#### Week 7: User Experience & Debugging Tools
**Deliverables:**
- Enhanced warning messages with clear reasoning
- Debugging tools for predicate evaluation
- Documentation for troubleshooting

**Example Warning Output:**
```
Warning: Field 'enablePrivateNodes' may require deleteBeforeReplace
├─ Reason: Change detected in private nodes configuration
├─ Condition: Instance has attached network interfaces that cannot be shared
├─ Recommendation: Set 'deleteBeforeReplace: true' on this resource
├─ Alternative: Remove conflicting attachments before update
└─ Learn more: https://pulumi.com/docs/delete-before-replace-conditions

To automatically apply this recommendation, set:
PULUMI_EXPERIMENTAL_AUTO_DELETE_BEFORE_REPLACE=true
```

**Files to Modify:**
- Warning and error message systems
- Documentation generation
- User-facing help text

**Risk Level:** Low - UX improvements only

### Phase 4: Documentation & Validation (2 weeks)

#### Week 8: Documentation Generation Enhancement
**Deliverables:**
- Enhance tfgen to document conditional delete-before-replace behavior
- Add field-level documentation annotations
- Update provider documentation with examples

**Files to Modify:**
- `pkg/tfgen/generate.go` - Add conditional documentation
- `pkg/tfgen/docs.go` - Enhanced doc processing
- Documentation templates

**Risk Level:** Low - Documentation improvements only

#### Week 9: Comprehensive Testing & Validation
**Deliverables:**
- End-to-end integration tests
- Cross-provider compatibility testing
- Performance impact assessment
- Regression testing suite

**Test Categories:**
1. **Unit Tests**: Individual predicate logic
2. **Integration Tests**: Full diff computation with predicates
3. **Cross-Tests**: Compare Terraform vs Pulumi behavior
4. **Provider Tests**: Real-world scenarios with GCP/Linode
5. **Performance Tests**: Impact on diff computation time
6. **Regression Tests**: Ensure existing providers unaffected

**Risk Level:** Critical - Final validation before release

## Detailed Technical Specifications

### Predicate Implementation Details

#### HasAttachedResources Predicate
```go
type HasAttachedResourcesConfig struct {
    ResourceTypes   []string  // Types to check: "networkInterface", "disk", etc.
    CheckConflicts  bool      // Whether to verify potential conflicts
    MinimumCount    int       // Minimum number that triggers condition
    MaximumCount    int       // Maximum number that triggers condition
}

func (p *HasAttachedResourcesPredicate) Evaluate(
    config HasAttachedResourcesConfig,
    old, new resource.PropertyMap,
    meta ResourceMetadata,
) bool {
    // Implementation:
    // 1. Query attached resources from metadata
    // 2. Filter by specified types
    // 3. Check count against min/max thresholds
    // 4. Optionally verify conflict potential
}
```

#### FieldChanged Predicate
```go
type FieldChangedConfig struct {
    Field       string      // Field path (supports nested: "config.network.private")
    IgnoreOrder bool        // For arrays/lists, ignore order changes
    Threshold   interface{} // Minimum change threshold for numeric fields
}

func (p *FieldChangedPredicate) Evaluate(
    config FieldChangedConfig,
    old, new resource.PropertyMap,
) bool {
    // Implementation:
    // 1. Extract field values from old and new maps
    // 2. Compare values with appropriate logic
    // 3. Apply threshold/ignore rules as configured
}
```

### Runtime Integration

#### Diff Logic Integration
```go
// Location: pkg/tfbridge/detailed_diff.go
func (p *Provider) buildDetailedDiff(
    olds, news resource.PropertyMap,
    res *ResourceInfo,
) (*DiffResult, error) {
    // ... existing diff logic ...
    
    // NEW: Evaluate delete-before-replace predicates
    deleteBeforeReplaceRecommendation, err := p.evaluateDeleteBeforeReplaceConditions(
        olds, news, res,
    )
    if err != nil {
        return nil, err
    }
    
    // Add warnings to diff result
    if deleteBeforeReplaceRecommendation.ShouldWarn {
        result.Warnings = append(result.Warnings, deleteBeforeReplaceRecommendation.Warning)
    }
    
    // Apply automatic behavior if experimental flag is enabled
    if deleteBeforeReplaceRecommendation.ShouldAutoApply && isExperimentalFlagEnabled() {
        result.DeleteBeforeReplace = true
        result.Reasoning = append(result.Reasoning, deleteBeforeReplaceRecommendation.Reasoning...)
    }
    
    return result, nil
}
```

#### Warning System Enhancement
```go
type DeleteBeforeReplaceWarning struct {
    Field       string
    Reason      string
    Conditions  []string
    Suggestions []string
    LearnMoreURL string
}

func (w *DeleteBeforeReplaceWarning) Format() string {
    // Generate user-friendly warning message with tree structure
}
```

## Testing Strategy

### Test Coverage Requirements

1. **Unit Test Coverage**: >90% for all new predicate and evaluation code
2. **Integration Test Coverage**: >80% for diff logic integration
3. **Cross-Provider Tests**: Verify consistent behavior across GCP, Linode, and other providers
4. **Performance Tests**: Ensure <5% impact on diff computation time
5. **Regression Tests**: Zero regression in existing provider behavior

### Test Scenarios

#### Unit Test Scenarios
- Individual predicate evaluation with various inputs
- Logic combination (AND, OR, NOT) with multiple predicates
- Edge cases: empty fields, null values, complex nested structures
- Error handling: invalid configurations, missing metadata

#### Integration Test Scenarios
- Real resource diffs with predicate evaluation
- Warning generation and formatting
- Experimental flag behavior
- Cross-framework consistency (SDKv2 vs Plugin Framework)

#### Provider-Specific Test Scenarios
- **GCP**: Compute instances with various attachment configurations
- **Linode**: Different instance types and configuration changes
- **Generic**: Common patterns applicable to multiple providers

### Performance Benchmarks

Target performance impact:
- Predicate evaluation: <1ms per field
- Overall diff computation: <5% increase
- Memory usage: <10% increase during diff computation

## Risk Assessment & Mitigation

### High-Risk Areas

1. **Core Diff Logic Changes**
   - **Risk**: Breaking existing provider behavior
   - **Mitigation**: Extensive regression testing, phased rollout
   - **Rollback Plan**: Feature flag to disable predicate evaluation

2. **Cross-Framework Consistency**
   - **Risk**: Different behavior between SDKv2 and Plugin Framework
   - **Mitigation**: Shared evaluation engine, comprehensive cross-tests
   - **Monitoring**: Automated tests comparing both frameworks

3. **Performance Impact**
   - **Risk**: Slowing down diff computation
   - **Mitigation**: Performance benchmarks, lazy evaluation, caching
   - **Monitoring**: Continuous performance regression testing

### Medium-Risk Areas

1. **Predicate Configuration Complexity**
   - **Risk**: Users struggling with configuration
   - **Mitigation**: Clear documentation, validation, helpful error messages
   - **Support**: Examples and troubleshooting guides

2. **Provider-Specific Logic**
   - **Risk**: Incorrect cloud provider assumptions
   - **Mitigation**: Collaboration with provider teams, extensive testing
   - **Validation**: Real-world scenario testing

### Low-Risk Areas

1. **Documentation Generation**
   - **Risk**: Incorrect or unclear documentation
   - **Mitigation**: Review process, user testing
   - **Impact**: Documentation-only, easily fixable

2. **Warning Messages**
   - **Risk**: Confusing or unhelpful warnings
   - **Mitigation**: User experience testing, iterative improvement
   - **Impact**: UX only, no functional impact

## Success Metrics

### Phase 1 Success Criteria
- [ ] Predicate evaluation framework implemented and tested
- [ ] Zero regression in existing provider functionality
- [ ] Unit test coverage >90% for new code
- [ ] Performance impact <2% on diff computation

### Phase 2 Success Criteria
- [ ] GCP and Linode provider integrations working correctly
- [ ] Warning system generating helpful recommendations
- [ ] User feedback indicates warnings are accurate >80% of the time
- [ ] Cross-framework behavior is consistent

### Phase 3 Success Criteria
- [ ] Experimental automatic behavior functions correctly
- [ ] Feature flag system robust and safe
- [ ] Extensive logging provides clear audit trail
- [ ] No unintended automatic applications

### Phase 4 Success Criteria
- [ ] Documentation clearly explains new functionality
- [ ] User adoption of conditional delete-before-replace increases
- [ ] Support tickets related to replacement failures decrease
- [ ] Community feedback is positive

### Long-term Success Metrics (6+ months)
- [ ] Reduced replacement-related failures in production
- [ ] Increased user satisfaction with replacement behavior
- [ ] Successful adoption by additional providers beyond GCP/Linode
- [ ] Feature graduates from experimental to stable

## Migration Path & Backwards Compatibility

### Backwards Compatibility Guarantees
1. **Existing Configurations**: All current `DeleteBeforeReplace: true` configurations continue working unchanged
2. **No Breaking Changes**: New functionality is purely additive
3. **Opt-in Behavior**: Predicate-based evaluation only occurs when explicitly configured
4. **Default Behavior**: Unchanged for resources not using new predicates

### Migration Strategy
1. **Phase 1**: New functionality available but not automatically applied
2. **Phase 2**: Providers can opt-in to predicate-based configurations
3. **Phase 3**: Community feedback and refinement
4. **Phase 4**: Stable release with full documentation

### Deprecation Path (Future)
- No immediate deprecation planned
- Boolean `DeleteBeforeReplace` remains supported indefinitely
- Future versions may encourage migration to predicate-based approach

## Resource Requirements

### Development Team
- **Lead Developer**: 1 FTE for full 9-week duration
- **Provider Specialists**: 0.5 FTE each for GCP and Linode integration (weeks 4-5)
- **Testing Engineer**: 0.5 FTE for comprehensive testing (weeks 6-9)
- **Technical Writer**: 0.25 FTE for documentation (weeks 8-9)

### Infrastructure Requirements
- Enhanced CI/CD pipeline for cross-provider testing
- Performance monitoring infrastructure
- Extended test environments for multiple providers

### Review & Approval Process
- Architecture review at end of Phase 1
- Security review before Phase 3 (experimental behavior)
- Provider team approval for each provider integration
- Documentation review before final release

## Conclusion

This implementation plan provides a comprehensive approach to adding field-level conditional `DeleteBeforeReplace` functionality to the Pulumi Terraform Bridge. The predicate-based system offers the flexibility of "truthiness functions" while maintaining transparency, debuggability, and cross-language compatibility.

The phased approach minimizes risk by starting with warnings and progressing to automatic behavior only after validation. The extensive testing strategy ensures compatibility and performance, while the migration path preserves backwards compatibility.

This feature will significantly improve the user experience for complex resource replacement scenarios while providing a foundation for future enhancements to the bridge's intelligence and automation capabilities.