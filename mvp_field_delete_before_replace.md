# MVP: Field-Level DeleteBeforeReplace Implementation

## Overview

This document outlines a Minimum Viable Product (MVP) implementation for field-level conditional `DeleteBeforeReplace` functionality. The MVP focuses on proving the core concept with minimal complexity and risk.

## MVP Scope

### What's Included
- **Single Predicate**: `FieldChanged` predicate only
- **Warning-Only Behavior**: No automatic replacement behavior changes
- **Multiple Provider Support**: GCP and Linode examples
- **Basic Integration**: Core diff logic integration with predicate framework
- **Predicate-Based Configuration**: Foundation for future expansion

### What's Excluded (Future Phases)
- Multiple predicate types (`HasAttachedResources`, `FieldEquals`, etc.)
- Automatic delete-before-replace application (`AutoApply` behavior)
- Complex predicate logic combinations (AND/OR)
- Plugin Framework support (SDKv2 only for MVP)
- Advanced predicates requiring resource metadata

## Technical Design

### Schema Changes

```go
// Location: pkg/tfbridge/info/info.go

// Add to existing Schema struct
type Schema struct {
    // ... existing fields ...
    ForceNew                *bool
    DeleteBeforeReplace     *bool                           // Simple boolean case (unchanged)
    DeleteBeforeReplaceFunc *DeleteBeforeReplaceEvaluator  // NEW: Predicate-based evaluation
}

// MVP: Predicate-based configuration (simplified for MVP)
type DeleteBeforeReplaceEvaluator struct {
    Predicates []DeleteBeforeReplacePredicate  // List of conditions to evaluate
    Logic      PredicateLogic                  // How to combine predicates (MVP: only "AND")
    Behavior   DeleteBeforeReplaceBehavior     // MVP: only WarnUser
    Reasoning  string                          // Human-readable explanation
}

type DeleteBeforeReplacePredicate struct {
    Type        string                 // MVP: only "fieldChanged"
    Config      map[string]interface{} // Predicate-specific configuration
    Description string                 // Human-readable description for debugging
}

type PredicateLogic struct {
    Op   string  // MVP: only "AND"
}

type DeleteBeforeReplaceBehavior int
const (
    WarnUser  DeleteBeforeReplaceBehavior = iota  // MVP: only warn user
    // AutoApply                                  // Future: automatic application
)
```

### Core Implementation

```go
// Location: pkg/tfbridge/predicate_evaluator.go (NEW FILE)

package tfbridge

import (
    "fmt"
    "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// MVP: Predicate evaluation engine (simplified)
type PredicateEvaluator struct{}

func NewPredicateEvaluator() *PredicateEvaluator {
    return &PredicateEvaluator{}
}

// Evaluate runs the predicate evaluation for a field
func (pe *PredicateEvaluator) Evaluate(
    evaluator *DeleteBeforeReplaceEvaluator,
    old, new resource.PropertyMap,
    meta ResourceMetadata,
) (*EvaluationResult, error) {
    
    if evaluator == nil {
        return nil, nil
    }
    
    // MVP: Only support single "fieldChanged" predicate
    var matchedPredicates []string
    var reasoning []string
    
    for _, predicate := range evaluator.Predicates {
        if predicate.Type == "fieldChanged" {
            matched, reason, err := pe.evaluateFieldChanged(predicate, old, new)
            if err != nil {
                return nil, err
            }
            if matched {
                matchedPredicates = append(matchedPredicates, predicate.Description)
                reasoning = append(reasoning, reason)
            }
        }
    }
    
    // MVP: Simple AND logic - all predicates must match
    shouldWarn := len(matchedPredicates) == len(evaluator.Predicates) && len(matchedPredicates) > 0
    
    return &EvaluationResult{
        ShouldDeleteBeforeReplace: shouldWarn,
        Reasoning:                 reasoning,
        MatchedPredicates:        matchedPredicates,
    }, nil
}

// MVP: Field changed predicate implementation
func (pe *PredicateEvaluator) evaluateFieldChanged(
    predicate DeleteBeforeReplacePredicate,
    old, new resource.PropertyMap,
) (bool, string, error) {
    
    fieldName, ok := predicate.Config["field"].(string)
    if !ok {
        return false, "", fmt.Errorf("fieldChanged predicate missing 'field' config")
    }
    
    oldValue := old[resource.PropertyKey(fieldName)]
    newValue := new[resource.PropertyKey(fieldName)]
    
    changed := fieldChanged(oldValue, newValue)
    reason := fmt.Sprintf("Field '%s' changed from %v to %v", fieldName, oldValue, newValue)
    
    return changed, reason, nil
}

// Helper function to detect field changes
func fieldChanged(old, new resource.PropertyValue) bool {
    // Handle nil cases
    if old.IsNull() && new.IsNull() {
        return false
    }
    if old.IsNull() != new.IsNull() {
        return true
    }
    
    // Compare values
    return !old.DeepEquals(new)
}

// Supporting types
type ResourceMetadata struct {
    ProviderName string
    ResourceType string
    // MVP: Minimal metadata, extended in future phases
}

type EvaluationResult struct {
    ShouldDeleteBeforeReplace bool
    Reasoning                 []string  // Step-by-step reasoning
    MatchedPredicates        []string  // Which predicates matched
}

// Warning structure
type DeleteBeforeReplaceWarning struct {
    Field          string
    Reason         string
    Recommendation string
}

func (w *DeleteBeforeReplaceWarning) String() string {
    return fmt.Sprintf(
        "Warning: Field '%s' may require deleteBeforeReplace\n"+
        "Reason: %s\n"+
        "Recommendation: %s",
        w.Field, w.Reason, w.Recommendation,
    )
}
```

### Integration with Diff Logic

```go
// Location: pkg/tfbridge/detailed_diff.go
// Add to existing buildDetailedDiff function

func (p *Provider) buildDetailedDiff(
    olds, news resource.PropertyMap,
    tfs shim.ResourceMap,
    ps *SchemaInfo,
    res *ResourceInfo,
) (*pulumirpc.DiffResponse, error) {
    
    // ... existing diff logic ...
    
    // NEW: Check for field-level delete-before-replace warnings
    warnings := p.checkFieldDeleteBeforeReplaceWarnings(olds, news, res)
    
    // Add warnings to response (implementation depends on existing warning system)
    for _, warning := range warnings {
        // Add to logging or diff response metadata
        glog.Warningf("DeleteBeforeReplace recommendation: %s", warning.String())
    }
    
    // ... rest of existing logic ...
    
    return result, nil
}

// NEW: Check all fields for delete-before-replace warnings
func (p *Provider) checkFieldDeleteBeforeReplaceWarnings(
    olds, news resource.PropertyMap,
    res *ResourceInfo,
) []*DeleteBeforeReplaceWarning {
    
    var warnings []*DeleteBeforeReplaceWarning
    checker := NewFieldDeleteBeforeReplaceChecker()
    
    // Iterate through all fields that have warning configuration
    for fieldName, schemaInfo := range res.Fields {
        if schemaInfo.WarnDeleteBeforeReplace == nil {
            continue
        }
        
        oldValue := olds[resource.PropertyKey(fieldName)]
        newValue := news[resource.PropertyKey(fieldName)]
        
        if warning := checker.CheckField(fieldName, oldValue, newValue, schemaInfo.WarnDeleteBeforeReplace); warning != nil {
            warnings = append(warnings, warning)
        }
    }
    
    return warnings
}
```

## Example Usage

### Provider Configuration

#### GCP Provider Example

```go
// Example: In a GCP provider configuration file

package main

import (
    "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
    "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func configureGCPProvider() *tfbridge.ProviderInfo {
    return &tfbridge.ProviderInfo{
        // ... existing configuration ...
        
        Resources: map[string]*tfbridge.ResourceInfo{
            "google_compute_instance": {
                Tok: "gcp:compute/instance:Instance",
                Fields: map[string]*tfbridge.SchemaInfo{
                    "enable_private_nodes": {
                        // Existing ForceNew behavior (unchanged)
                        ForceNew: &[]bool{true}[0],
                        
                        // NEW: Predicate-based delete-before-replace warning
                        DeleteBeforeReplaceFunc: &tfbridge.DeleteBeforeReplaceEvaluator{
                            Predicates: []tfbridge.DeleteBeforeReplacePredicate{
                                {
                                    Type: "fieldChanged",
                                    Config: map[string]interface{}{
                                        "field": "enable_private_nodes",
                                    },
                                    Description: "Private nodes setting is changing",
                                },
                            },
                            Logic: tfbridge.PredicateLogic{Op: "AND"},
                            Behavior: tfbridge.WarnUser,
                            Reasoning: "Private node configuration changes may conflict with attached network interfaces",
                        },
                    },
                    "machine_type": {
                        ForceNew: &[]bool{true}[0],
                        DeleteBeforeReplaceFunc: &tfbridge.DeleteBeforeReplaceEvaluator{
                            Predicates: []tfbridge.DeleteBeforeReplacePredicate{
                                {
                                    Type: "fieldChanged",
                                    Config: map[string]interface{}{
                                        "field": "machine_type",
                                    },
                                    Description: "Machine type is changing",
                                },
                            },
                            Logic: tfbridge.PredicateLogic{Op: "AND"},
                            Behavior: tfbridge.WarnUser,
                            Reasoning: "Machine type changes require instance replacement and may conflict with persistent resources",
                        },
                    },
                },
            },
        },
    }
}
```

#### Linode Provider Example

```go
// Example: In a Linode provider configuration file

func configureLinodeProvider() *tfbridge.ProviderInfo {
    return &tfbridge.ProviderInfo{
        // ... existing configuration ...
        
        Resources: map[string]*tfbridge.ResourceInfo{
            "linode_instance": {
                Tok: "linode:index/instance:Instance",
                Fields: map[string]*tfbridge.SchemaInfo{
                    "root_pass": {
                        ForceNew: &[]bool{true}[0],
                        DeleteBeforeReplaceFunc: &tfbridge.DeleteBeforeReplaceEvaluator{
                            Predicates: []tfbridge.DeleteBeforeReplacePredicate{
                                {
                                    Type: "fieldChanged",
                                    Config: map[string]interface{}{
                                        "field": "root_pass",
                                    },
                                    Description: "Root password is changing",
                                },
                            },
                            Logic: tfbridge.PredicateLogic{Op: "AND"},
                            Behavior: tfbridge.WarnUser,
                            Reasoning: "Root password changes may require delete-before-replace to avoid configuration conflicts",
                        },
                    },
                },
            },
        },
    }
}
```

### Expected User Experience

When a user changes the `enable_private_nodes` field on a GCP compute instance:

```bash
$ pulumi preview

Previewing update (dev):
     Type                         Name                    Plan       
     pulumi:pulumi:Stack         my-stack-dev             
 ~   └─ gcp:compute:Instance     my-instance             update     

Resources:
    ~ 1 to update
    1 unchanged

Warning: Field 'enable_private_nodes' may require deleteBeforeReplace
Reason: Private node configuration changes may conflict with attached network interfaces
Recommendation: Consider setting 'deleteBeforeReplace: true' on this resource

Do you want to perform this update?
```

## Test Implementation

### Unit Tests

```go
// Location: pkg/tfbridge/field_delete_before_replace_test.go (NEW FILE)

package tfbridge

import (
    "testing"
    
    "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
    "github.com/stretchr/testify/assert"
)

func TestFieldDeleteBeforeReplaceChecker_NoConfigReturnsNil(t *testing.T) {
    checker := NewFieldDeleteBeforeReplaceChecker()
    
    oldValue := resource.NewStringProperty("old-value")
    newValue := resource.NewStringProperty("new-value")
    
    warning := checker.CheckField("test_field", oldValue, newValue, nil)
    
    assert.Nil(t, warning)
}

func TestFieldDeleteBeforeReplaceChecker_DisabledReturnsNil(t *testing.T) {
    checker := NewFieldDeleteBeforeReplaceChecker()
    config := &FieldDeleteBeforeReplaceWarning{
        Enabled: false,
        Reason:  "Test reason",
    }
    
    oldValue := resource.NewStringProperty("old-value")
    newValue := resource.NewStringProperty("new-value")
    
    warning := checker.CheckField("test_field", oldValue, newValue, config)
    
    assert.Nil(t, warning)
}

func TestFieldDeleteBeforeReplaceChecker_NoChangeReturnsNil(t *testing.T) {
    checker := NewFieldDeleteBeforeReplaceChecker()
    config := &FieldDeleteBeforeReplaceWarning{
        Enabled: true,
        Reason:  "Test reason",
    }
    
    sameValue := resource.NewStringProperty("same-value")
    
    warning := checker.CheckField("test_field", sameValue, sameValue, config)
    
    assert.Nil(t, warning)
}

func TestFieldDeleteBeforeReplaceChecker_FieldChangedReturnsWarning(t *testing.T) {
    checker := NewFieldDeleteBeforeReplaceChecker()
    config := &FieldDeleteBeforeReplaceWarning{
        Enabled: true,
        Reason:  "Field change may cause conflicts",
    }
    
    oldValue := resource.NewStringProperty("old-value")
    newValue := resource.NewStringProperty("new-value")
    
    warning := checker.CheckField("test_field", oldValue, newValue, config)
    
    assert.NotNil(t, warning)
    assert.Equal(t, "test_field", warning.Field)
    assert.Equal(t, "Field change may cause conflicts", warning.Reason)
    assert.Contains(t, warning.Recommendation, "deleteBeforeReplace")
}

func TestFieldDeleteBeforeReplaceChecker_NullToValueReturnsWarning(t *testing.T) {
    checker := NewFieldDeleteBeforeReplaceChecker()
    config := &FieldDeleteBeforeReplaceWarning{
        Enabled: true,
        Reason:  "Test reason",
    }
    
    oldValue := resource.NewNullProperty()
    newValue := resource.NewStringProperty("new-value")
    
    warning := checker.CheckField("test_field", oldValue, newValue, config)
    
    assert.NotNil(t, warning)
}

func TestFieldDeleteBeforeReplaceChecker_ValueToNullReturnsWarning(t *testing.T) {
    checker := NewFieldDeleteBeforeReplaceChecker()
    config := &FieldDeleteBeforeReplaceWarning{
        Enabled: true,
        Reason:  "Test reason",
    }
    
    oldValue := resource.NewStringProperty("old-value")
    newValue := resource.NewNullProperty()
    
    warning := checker.CheckField("test_field", oldValue, newValue, config)
    
    assert.NotNil(t, warning)
}

func TestFieldDeleteBeforeReplaceChecker_BothNullReturnsNil(t *testing.T) {
    checker := NewFieldDeleteBeforeReplaceChecker()
    config := &FieldDeleteBeforeReplaceWarning{
        Enabled: true,
        Reason:  "Test reason",
    }
    
    oldValue := resource.NewNullProperty()
    newValue := resource.NewNullProperty()
    
    warning := checker.CheckField("test_field", oldValue, newValue, config)
    
    assert.Nil(t, warning)
}

func TestFieldChanged_StringValues(t *testing.T) {
    tests := []struct {
        name     string
        old      resource.PropertyValue
        new      resource.PropertyValue
        expected bool
    }{
        {
            name:     "same string values",
            old:      resource.NewStringProperty("test"),
            new:      resource.NewStringProperty("test"),
            expected: false,
        },
        {
            name:     "different string values",
            old:      resource.NewStringProperty("old"),
            new:      resource.NewStringProperty("new"),
            expected: true,
        },
        {
            name:     "empty to non-empty",
            old:      resource.NewStringProperty(""),
            new:      resource.NewStringProperty("value"),
            expected: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := fieldChanged(tt.old, tt.new)
            assert.Equal(t, tt.expected, result)
        })
    }
}

func TestFieldChanged_BooleanValues(t *testing.T) {
    tests := []struct {
        name     string
        old      resource.PropertyValue
        new      resource.PropertyValue
        expected bool
    }{
        {
            name:     "same boolean values true",
            old:      resource.NewBoolProperty(true),
            new:      resource.NewBoolProperty(true),
            expected: false,
        },
        {
            name:     "same boolean values false",
            old:      resource.NewBoolProperty(false),
            new:      resource.NewBoolProperty(false),
            expected: false,
        },
        {
            name:     "different boolean values",
            old:      resource.NewBoolProperty(true),
            new:      resource.NewBoolProperty(false),
            expected: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := fieldChanged(tt.old, tt.new)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Integration Test

```go
// Location: pkg/tfbridge/detailed_diff_delete_before_replace_test.go (NEW FILE)

package tfbridge

import (
    "testing"
    
    "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestProvider_checkFieldDeleteBeforeReplaceWarnings_NoWarningsWhenNoConfig(t *testing.T) {
    provider := &Provider{}
    
    olds := resource.PropertyMap{
        "field1": resource.NewStringProperty("old-value"),
    }
    news := resource.PropertyMap{
        "field1": resource.NewStringProperty("new-value"),
    }
    
    res := &ResourceInfo{
        Fields: map[string]*SchemaInfo{
            "field1": {
                // No WarnDeleteBeforeReplace configuration
            },
        },
    }
    
    warnings := provider.checkFieldDeleteBeforeReplaceWarnings(olds, news, res)
    
    assert.Empty(t, warnings)
}

func TestProvider_checkFieldDeleteBeforeReplaceWarnings_GeneratesWarningForConfiguredField(t *testing.T) {
    provider := &Provider{}
    
    olds := resource.PropertyMap{
        "enable_private_nodes": resource.NewBoolProperty(false),
    }
    news := resource.PropertyMap{
        "enable_private_nodes": resource.NewBoolProperty(true),
    }
    
    res := &ResourceInfo{
        Fields: map[string]*SchemaInfo{
            "enable_private_nodes": {
                WarnDeleteBeforeReplace: &FieldDeleteBeforeReplaceWarning{
                    Enabled: true,
                    Reason:  "Private node changes may conflict with network interfaces",
                },
            },
        },
    }
    
    warnings := provider.checkFieldDeleteBeforeReplaceWarnings(olds, news, res)
    
    require.Len(t, warnings, 1)
    assert.Equal(t, "enable_private_nodes", warnings[0].Field)
    assert.Equal(t, "Private node changes may conflict with network interfaces", warnings[0].Reason)
    assert.Contains(t, warnings[0].Recommendation, "deleteBeforeReplace")
}

func TestProvider_checkFieldDeleteBeforeReplaceWarnings_NoWarningWhenFieldUnchanged(t *testing.T) {
    provider := &Provider{}
    
    sameValue := resource.NewBoolProperty(true)
    olds := resource.PropertyMap{
        "enable_private_nodes": sameValue,
    }
    news := resource.PropertyMap{
        "enable_private_nodes": sameValue,
    }
    
    res := &ResourceInfo{
        Fields: map[string]*SchemaInfo{
            "enable_private_nodes": {
                WarnDeleteBeforeReplace: &FieldDeleteBeforeReplaceWarning{
                    Enabled: true,
                    Reason:  "Test reason",
                },
            },
        },
    }
    
    warnings := provider.checkFieldDeleteBeforeReplaceWarnings(olds, news, res)
    
    assert.Empty(t, warnings)
}

func TestProvider_checkFieldDeleteBeforeReplaceWarnings_MultipleFields(t *testing.T) {
    provider := &Provider{}
    
    olds := resource.PropertyMap{
        "enable_private_nodes": resource.NewBoolProperty(false),
        "machine_type":         resource.NewStringProperty("n1-standard-1"),
        "other_field":          resource.NewStringProperty("unchanged"),
    }
    news := resource.PropertyMap{
        "enable_private_nodes": resource.NewBoolProperty(true),
        "machine_type":         resource.NewStringProperty("n1-standard-2"),
        "other_field":          resource.NewStringProperty("unchanged"),
    }
    
    res := &ResourceInfo{
        Fields: map[string]*SchemaInfo{
            "enable_private_nodes": {
                WarnDeleteBeforeReplace: &FieldDeleteBeforeReplaceWarning{
                    Enabled: true,
                    Reason:  "Private node changes may conflict",
                },
            },
            "machine_type": {
                WarnDeleteBeforeReplace: &FieldDeleteBeforeReplaceWarning{
                    Enabled: true,
                    Reason:  "Machine type changes require special handling",
                },
            },
            "other_field": {
                // No warning configuration
            },
        },
    }
    
    warnings := provider.checkFieldDeleteBeforeReplaceWarnings(olds, news, res)
    
    require.Len(t, warnings, 2)
    
    // Should have warnings for both changed fields with configuration
    fieldNames := []string{warnings[0].Field, warnings[1].Field}
    assert.Contains(t, fieldNames, "enable_private_nodes")
    assert.Contains(t, fieldNames, "machine_type")
}
```

## Implementation Steps

### Phase 1: Foundation (Week 1)
1. Add `FieldDeleteBeforeReplaceWarning` struct to `pkg/tfbridge/info/info.go`
2. Create `pkg/tfbridge/field_delete_before_replace.go` with core logic
3. Add unit tests for field change detection and warning generation

### Phase 2: Integration (Week 2)
1. Integrate warning check into `pkg/tfbridge/detailed_diff.go`
2. Add integration tests
3. Test with simple scenarios

### Phase 3: Provider Configuration (Week 3)
1. Configure GCP provider with example field warnings
2. End-to-end testing with real GCP resources
3. Documentation and examples

## Benefits of This MVP

1. **Low Risk**: Only adds warnings, no behavior changes
2. **Validates Architecture**: Proves the field-level concept works
3. **Quick Implementation**: Simple scope, ~3 weeks
4. **Immediate Value**: Helps users understand when deleteBeforeReplace might be needed
5. **Foundation for Future**: Easy to extend with more predicates and automatic behavior

## Success Criteria

- [ ] Field-level warning configuration works
- [ ] Warnings appear when configured fields change
- [ ] No warnings when fields don't change
- [ ] Zero regression in existing provider behavior
- [ ] Clean integration with existing diff logic
- [ ] Comprehensive test coverage (>90%)

This MVP provides a solid foundation for the full predicate-based system while delivering immediate value to users and validating the core architectural concepts.