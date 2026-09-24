# Provider Inconsistency Detection Implementation Guide (Revised)

## Overview

This implementation guide outlines how to add a feature to detect and report when upstream Terraform providers produce inconsistent results after apply operations. This addresses issue #2413 where Terraform providers with bugs that trigger an "inconsistent result of apply" error don't properly surface this information to Pulumi users.

The implementation will:
1. Add comparison logic between planned state and actual state after create/update operations 
2. Log detected inconsistencies as warnings to help users troubleshoot
3. Make this feature configurable with environment variables to ensure backward compatibility
4. Filter out known/expected inconsistencies to reduce noise

## Background

Terraform and OpenTofu detect inconsistencies between the planned state and actual state after an apply operation. When detected, they display an error message like:

```
Error: Provider produced inconsistent result after apply

When applying changes to resource_type.resource_name, provider
produced an unexpected new value:
.some_property: was cty.ListValEmpty(cty.String), but now null.

This is a bug in the provider, which should be reported in the provider's own issue
tracker.
```

Currently, Pulumi's Terraform bridge doesn't perform this detection, so users just experience inexplicable diffs on subsequent operations when this issue occurs.

## OpenTofu Implementation Analysis

OpenTofu implements inconsistency detection by comparing the planned state against the final state returned after an apply operation. Key aspects of their implementation:

1. The detection happens in the apply phase, after a resource has been created or updated
2. The comparison is done attribute-by-attribute, looking for differences between planned and actual values
3. The error message includes specific paths to inconsistent values and shows both the expected and actual values
4. The issue is reported as an error but doesn't prevent the apply operation from completing
5. The message clearly indicates this is a provider bug, not a user configuration error

OpenTofu's implementation handles special cases like:
- Comparing empty collections vs. null values (a common source of inconsistencies)
- Type differences (e.g., string representation vs. actual type)
- Nested attributes in complex data structures

## Implementation Lessons Learned

### Key Design Considerations

1. **Post-Apply Analysis Only**: The inconsistency detection should happen only after the resource has been successfully created or updated. It should not modify any core behavior or affect the creation/update process itself.

2. **No Side Effects**: The detection code must be purely analytical. It should not modify any state, interfere with naming, auto-aliasing, or other bridge features.

3. **Warning Only**: Detected inconsistencies should be logged as warnings only, not errors. This is a change from OpenTofu's approach to avoid disrupting Pulumi workflows while still providing visibility.

4. **Separate from Core Logic**: Keep the feature logic isolated from core bridge functionality to avoid introducing bugs or regressions.

5. **Minimal Dependencies**: The detection code should have minimal dependencies on other bridge components to reduce the risk of unexpected interactions.

### Implementation Pitfalls to Avoid

1. **❌ DO NOT modify the resource state**: The implementation should never modify the state being returned by the provider, even if inconsistencies are detected. This would lead to unexpected behavior.

2. **❌ DO NOT alter the resource ID**: Never manipulate the resource ID or other crucial identifiers during inconsistency detection. This will break resources.

3. **❌ DO NOT interfere with autonaming or renaming**: The detection happens after these processes and should not attempt to modify them.

4. **❌ DO NOT use different output structures**: Keep the interface to inconsistency detection simple and focused.

5. **❌ DO NOT add excessive filtering too early**: Start with minimal filtering and add more based on real-world usage patterns.

### Recommended Implementation Approach

1. **Create New Files**: 
   - `pkg/tfbridge/inconsistency_detector.go`: Core detection logic for SDKv2 providers
   - `pkg/pf/tfbridge/inconsistency_detector.go`: Plugin Framework specific implementation
   - `pkg/tfbridge/inconsistency_filter.go`: Filtering logic for known false positives

2. **Integration Points**:
   - Add detection calls after successful creation in the `Create` method
   - Add detection calls after successful updates in the `Update` method
   - Do not modify any other parts of the code flow

3. **Keep Analysis Simple**:
   - Compare planned state with actual state
   - Filter out known false positives
   - Log inconsistencies as warnings

## Step-by-Step Implementation

1. **Define Configuration System**:
   ```go
   // In pkg/tfbridge/inconsistency_config.go
   const (
       EnvDetectInconsistentApply = "PULUMI_DETECT_INCONSISTENT_APPLY"
       EnvDetectInconsistentApplyDetail = "PULUMI_DETECT_INCONSISTENT_APPLY_DETAIL"
       EnvDetectInconsistentApplyResources = "PULUMI_DETECT_INCONSISTENT_APPLY_RESOURCES"
   )

   type InconsistencyConfig struct {
       Enabled bool
       DetailLevel string
       ResourceTypes map[string]bool
   }

   func GetInconsistencyConfig() InconsistencyConfig {
       // Parse environment variables
   }

   func ShouldDetectInconsistentApply(resourceType string) bool {
       // Check if enabled for this resource
   }
   ```

2. **Create Filter Interface**:
   ```go
   // In pkg/tfbridge/inconsistency_filter.go
   type InconsistencyFilter interface {
       ShouldIgnoreAttribute(resourceType, attrName string) bool
       ShouldIgnoreValueChange(resourceType, attrName string, plannedVal, actualVal interface{}) bool
   }

   func NewDefaultInconsistencyFilter() InconsistencyFilter {
       // Create default implementation with common filters
   }
   ```

3. **Implement Detection Logic**:
   ```go
   // In pkg/tfbridge/inconsistency_detector.go
   type InconsistencyDetail struct {
       Path string
       PlannedValue interface{}
       ActualValue interface{}
       Description string
   }

   func detectInconsistentApply(
       ctx context.Context,
       resourceType string,
       plannedState shim.InstanceState,
       actualState shim.InstanceState,
       filter InconsistencyFilter,
   ) []InconsistencyDetail {
       // Return detected inconsistencies
   }

   func formatInconsistencyMessage(resourceType string, details []InconsistencyDetail) string {
       // Format human-readable message
   }
   ```

4. **Add To Provider Create/Update Methods**:
   ```go
   // At the end of the Create/Update method, after a successful apply
   // but before returning the results
   if !preview && ShouldDetectInconsistentApply(resourceType) {
       inconsistencies := detectInconsistentApply(
           ctx, resourceType, plannedState, actualState, filter)
       
       if len(inconsistencies) > 0 {
           message := formatInconsistencyMessage(resourceType, inconsistencies)
           logger := getLogger(ctx)
           logger.Warn(message)
       }
   }
   ```

5. **Add Comprehensive Tests**:
   ```go
   // In pkg/tfbridge/inconsistency_detector_test.go
   func TestDetectInconsistentApply(t *testing.T) {
       // Test various inconsistency scenarios
   }

   func TestInconsistencyFiltering(t *testing.T) {
       // Test filtering behavior
   }
   ```

## Common Inconsistency Patterns

Based on analysis of OpenTofu issues and provider bugs, these patterns should be specifically addressed:

1. **Null vs. Empty Collections**: Many providers inconsistently return `null` instead of empty arrays/maps or vice versa
   ```
   .tags: was "[]", but now null
   ```

2. **Type Inconsistencies**: Particularly between string representations and actual types
   ```
   .count: was "5", but now 5
   ```

3. **Computed Attributes**: Providers sometimes unexpectedly add computed attributes
   ```
   .computed_attribute: was absent, but now "auto-generated-value"
   ```

4. **ID Format Changes**: Changes in formatting for resource IDs or ARNs
   ```
   .id: was "my-resource", but now "my_resource"
   ```

5. **Eventual Consistency Issues**: Resources that haven't fully propagated may return inconsistent results

## Documentation For Users

```markdown
## Detecting Provider Inconsistencies

To help identify issues with Terraform providers, Pulumi can detect and report inconsistencies
between the planned state and actual state after resource operations.

### Environment Variables

- `PULUMI_DETECT_INCONSISTENT_APPLY`: Set to `true` to enable detection
- `PULUMI_DETECT_INCONSISTENT_APPLY_DETAIL`: Controls detail level (`normal`, `debug`, `trace`)
- `PULUMI_DETECT_INCONSISTENT_APPLY_RESOURCES`: Optional comma-separated resource types to check

### Example Usage

```bash
# Enable detection for all resources
export PULUMI_DETECT_INCONSISTENT_APPLY=true

# Enable detailed logging for specific resources
export PULUMI_DETECT_INCONSISTENT_APPLY=true
export PULUMI_DETECT_INCONSISTENT_APPLY_DETAIL=debug
export PULUMI_DETECT_INCONSISTENT_APPLY_RESOURCES="aws_lambda_function,aws_iam_role"
```

When inconsistencies are detected, you'll see warning messages that can help
identify bugs in the provider. These may explain unexplained diffs you see on
subsequent operations.
```

## Test Plan

1. **Unit Tests**:
   - Test configuration parsing
   - Test detection logic with various examples
   - Test filtering system
   - Test specifically against known inconsistency patterns

2. **Integration Tests**:
   - Create a test provider that deliberately returns inconsistent states
   - Verify detection works with real provider interactions

3. **Manual Testing**:
   - Test with known problematic providers (AWS, Azure, GCP)
   - Verify minimal performance impact

## Performance Considerations

- Enable logging only when the feature is activated
- Add early-return checks to avoid unnecessary processing
- Implement sampling for large collections to reduce overhead
- Add depth limits for recursive comparisons
- Use memory-efficient comparison algorithms for large state objects

## Conclusion

Adding inconsistency detection will help users understand and troubleshoot issues with Terraform providers. By implementing this carefully with proper isolation from core bridge functionality, we can provide this feature without affecting existing functionality or introducing regressions. Following OpenTofu's approach for detection but using warnings instead of errors will maintain compatibility while improving user experience.