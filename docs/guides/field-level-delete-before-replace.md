# Field-Level Delete Before Replace Configuration

This guide explains how to configure field-level delete-before-replace predicates in your bridged Terraform provider.

## Overview

The Pulumi Terraform Bridge supports field-level conditional logic that can recommend or automatically apply `deleteBeforeReplace` based on resource state. This uses a predicate-based system to make intelligent decisions about when delete-before-replace is needed.

This feature is particularly useful for:
- Network configuration changes that conflict with attached interfaces
- Instance type changes that conflict with attached storage
- Conditional logic based on resource state (attachments, instance types, etc.)
- Any field change where conflicts depend on other resource properties

## Predicate-Based Configuration

### Basic Predicate Setup

Configure conditional delete-before-replace using predicates in your provider's `ResourceInfo`:

```go
package main

import (
    "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
    "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/info"
)

func Provider() *tfbridge.ProviderInfo {
    return &tfbridge.ProviderInfo{
        // ... other provider configuration ...
        
        Resources: map[string]*tfbridge.ResourceInfo{
            "aws_instance": {
                Tok: "aws:ec2/instance:Instance",
                Fields: map[string]*tfbridge.SchemaInfo{
                    "instance_type": {
                        // This field already forces replacement in Terraform
                        ForceNew: &[]bool{true}[0],
                        
                        // NEW: Predicate-based delete-before-replace logic
                        DeleteBeforeReplaceFunc: &tfbridge.DeleteBeforeReplaceEvaluator{
                            Predicates: []tfbridge.DeleteBeforeReplacePredicate{
                                {
                                    Type: "fieldChanged",
                                    Config: map[string]interface{}{
                                        "field": "instance_type",
                                    },
                                    Description: "Instance type is changing",
                                },
                                {
                                    Type: "hasAttachedResources",
                                    Config: map[string]interface{}{
                                        "resourceTypes": []string{"networkInterface", "ebsVolume"},
                                        "minimumCount": 1,
                                    },
                                    Description: "Instance has attached resources that may conflict",
                                },
                            },
                            Logic: tfbridge.PredicateLogic{Op: "AND"},
                            Behavior: tfbridge.WarnUser,
                            Reasoning: "Instance type changes may conflict with attached EBS volumes and network interfaces",
                        },
                    },
                },
            },
        },
    }
}
```

### Predicate Configuration Structure

```go
type DeleteBeforeReplaceEvaluator struct {
    Predicates []DeleteBeforeReplacePredicate  // List of conditions to evaluate
    Logic      PredicateLogic                  // How to combine predicates (AND, OR, NOT)
    Behavior   DeleteBeforeReplaceBehavior     // Warn vs Auto-apply
    Reasoning  string                          // Human-readable explanation
}

type DeleteBeforeReplacePredicate struct {
    Type        string                 // Predicate type: "fieldChanged", "hasAttachedResources", etc.
    Config      map[string]interface{} // Predicate-specific configuration
    Description string                 // Human-readable description for debugging
}

type PredicateLogic struct {
    Op   string  // "AND", "OR", "NOT"
    Expr string  // Complex expressions like "(A AND B) OR C"
}

type DeleteBeforeReplaceBehavior int
const (
    WarnUser  DeleteBeforeReplaceBehavior = iota  // Show warning to user
    AutoApply                                     // Automatically apply (experimental)
)
```

## Available Predicate Types

### FieldChanged Predicate

Detects when a specific field has changed between old and new resource state.

```go
{
    Type: "fieldChanged",
    Config: map[string]interface{}{
        "field": "instance_type",           // Field path (supports nested: "config.network.private")
        "ignoreOrder": false,               // For arrays/lists, ignore order changes
        "threshold": 0,                     // Minimum change threshold for numeric fields
    },
    Description: "Instance type field is changing",
}
```

### HasAttachedResources Predicate

Checks for attached or dependent resources that might conflict during replacement.

```go
{
    Type: "hasAttachedResources",
    Config: map[string]interface{}{
        "resourceTypes": []string{"networkInterface", "disk"},  // Types to check for
        "checkConflicts": true,                                 // Whether to verify potential conflicts
        "minimumCount": 1,                                      // Minimum number that triggers condition
        "maximumCount": 10,                                     // Maximum number that triggers condition
    },
    Description: "Check for network interfaces that cannot be shared",
}
```

### FieldEquals Predicate

Checks if a field equals specific values.

```go
{
    Type: "fieldEquals",
    Config: map[string]interface{}{
        "field": "type",                                    // Field to check
        "values": []string{"g6-standard-1", "g6-standard-2"},  // Values that trigger the condition
    },
    Description: "Instance type requires special handling",
}
```

### FieldMatches Predicate

Checks if a field matches a regex pattern.

```go
{
    Type: "fieldMatches",
    Config: map[string]interface{}{
        "field": "machine_type",        // Field to check
        "pattern": "^n1-standard-.*",   // Regex pattern
    },
    Description: "N1 standard instance types need special handling",
}
```

### ResourceCount Predicate

Checks the count of related resources.

```go
{
    Type: "resourceCount",
    Config: map[string]interface{}{
        "resourceType": "attachedDisk",  // Type of resource to count
        "operator": ">=",                // Comparison operator: >=, <=, ==, !=
        "count": 2,                      // Count to compare against
    },
    Description: "Instance has multiple attached disks",
}
```

## Field Selection Guidelines

### When to Add Warnings

Add `WarnDeleteBeforeReplace` configuration to fields that:

1. **Already trigger replacement** (`ForceNew: true` in Terraform schema)
2. **Have attachment conflicts** (network interfaces, storage, etc.)
3. **Commonly cause replacement failures** in real-world usage
4. **Would benefit from delete-before-replace** strategy

### When NOT to Add Warnings

Don't add warnings for fields that:

1. **Don't trigger replacement** (in-place updates only)
2. **Rarely cause conflicts** (simple configuration changes)
3. **Work reliably with create-before-replace** (the default strategy)
4. **Have very fast replacement times** (where delete-before-replace isn't helpful)

## Provider-Specific Examples

### Google Cloud Platform (GCP)

```go
"google_compute_instance": {
    Tok: "gcp:compute/instance:Instance",
    Fields: map[string]*tfbridge.SchemaInfo{
        "enable_private_nodes": {
            ForceNew: &[]bool{true}[0],
            DeleteBeforeReplaceFunc: &tfbridge.DeleteBeforeReplaceEvaluator{
                Predicates: []tfbridge.DeleteBeforeReplacePredicate{
                    {
                        Type: "fieldChanged",
                        Config: map[string]interface{}{
                            "field": "enable_private_nodes",
                        },
                        Description: "Private nodes setting is changing",
                    },
                    {
                        Type: "hasAttachedResources",
                        Config: map[string]interface{}{
                            "resourceTypes": []string{"networkInterface", "attachedDisk"},
                            "checkConflicts": true,
                            "minimumCount": 1,
                        },
                        Description: "Check for network interfaces that cannot be shared",
                    },
                },
                Logic: tfbridge.PredicateLogic{Op: "AND"},
                Behavior: tfbridge.WarnUser,
                Reasoning: "Private node changes may conflict with attached network interfaces",
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
                    {
                        Type: "hasAttachedResources",
                        Config: map[string]interface{}{
                            "resourceTypes": []string{"persistentDisk", "networkInterface"},
                            "minimumCount": 1,
                        },
                        Description: "Instance has attached persistent resources",
                    },
                },
                Logic: tfbridge.PredicateLogic{Op: "AND"},
                Behavior: tfbridge.WarnUser,
                Reasoning: "Machine type changes may conflict with persistent disks or network interfaces",
            },
        },
    },
},
```

### AWS

```go
"aws_instance": {
    Tok: "aws:ec2/instance:Instance",
    Fields: map[string]*tfbridge.SchemaInfo{
        "instance_type": {
            ForceNew: &[]bool{true}[0],
            DeleteBeforeReplaceFunc: &tfbridge.DeleteBeforeReplaceEvaluator{
                Predicates: []tfbridge.DeleteBeforeReplacePredicate{
                    {
                        Type: "fieldChanged",
                        Config: map[string]interface{}{
                            "field": "instance_type",
                        },
                        Description: "Instance type is changing",
                    },
                    {
                        Type: "hasAttachedResources",
                        Config: map[string]interface{}{
                            "resourceTypes": []string{"ebsVolume", "networkInterface"},
                            "minimumCount": 1,
                        },
                        Description: "Instance has attached EBS volumes or network interfaces",
                    },
                },
                Logic: tfbridge.PredicateLogic{Op: "AND"},
                Behavior: tfbridge.WarnUser,
                Reasoning: "Instance type changes may conflict with attached EBS volumes and network interfaces",
            },
        },
        "subnet_id": {
            ForceNew: &[]bool{true}[0],
            DeleteBeforeReplaceFunc: &tfbridge.DeleteBeforeReplaceEvaluator{
                Predicates: []tfbridge.DeleteBeforeReplacePredicate{
                    {
                        Type: "fieldChanged",
                        Config: map[string]interface{}{
                            "field": "subnet_id",
                        },
                        Description: "Subnet ID is changing",
                    },
                },
                Logic: tfbridge.PredicateLogic{Op: "AND"},
                Behavior: tfbridge.WarnUser,
                Reasoning: "Subnet changes require instance replacement and may conflict with network interface attachments",
            },
        },
    },
},
```

### Linode

```go
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
                    {
                        Type: "fieldEquals",
                        Config: map[string]interface{}{
                            "field": "type",
                            "values": []string{"g6-standard-1", "g6-standard-2", "g6-standard-4"},
                        },
                        Description: "Instance type requires special password handling",
                    },
                },
                Logic: tfbridge.PredicateLogic{Op: "AND"},
                Behavior: tfbridge.WarnUser,
                Reasoning: "Root password changes on these instance types require delete-before-replace to avoid configuration conflicts",
            },
        },
        "type": {
            ForceNew: &[]bool{true}[0],
            DeleteBeforeReplaceFunc: &tfbridge.DeleteBeforeReplaceEvaluator{
                Predicates: []tfbridge.DeleteBeforeReplacePredicate{
                    {
                        Type: "fieldChanged",
                        Config: map[string]interface{}{
                            "field": "type",
                        },
                        Description: "Instance type is changing",
                    },
                    {
                        Type: "hasAttachedResources",
                        Config: map[string]interface{}{
                            "resourceTypes": []string{"volume", "nodebalancer"},
                            "minimumCount": 1,
                        },
                        Description: "Instance has attached volumes or load balancer configurations",
                    },
                },
                Logic: tfbridge.PredicateLogic{Op: "AND"},
                Behavior: tfbridge.WarnUser,
                Reasoning: "Instance type changes may conflict with attached volumes or NodeBalancer configurations",
            },
        },
        "label": {
            // Example: Label changes that might affect DNS or monitoring
            DeleteBeforeReplaceFunc: &tfbridge.DeleteBeforeReplaceEvaluator{
                Predicates: []tfbridge.DeleteBeforeReplacePredicate{
                    {
                        Type: "fieldChanged",
                        Config: map[string]interface{}{
                            "field": "label",
                        },
                        Description: "Instance label is changing",
                    },
                    {
                        Type: "fieldMatches",
                        Config: map[string]interface{}{
                            "field": "label",
                            "pattern": "^prod-.*",
                        },
                        Description: "Production instance labels have special requirements",
                    },
                },
                Logic: tfbridge.PredicateLogic{Op: "AND"},
                Behavior: tfbridge.WarnUser,
                Reasoning: "Label changes on production instances should use delete-before-replace to ensure DNS and monitoring consistency",
            },
        },
    },
},
```

### Azure

```go
"azurerm_virtual_machine": {
    Tok: "azure:compute/virtualMachine:VirtualMachine",
    Fields: map[string]*tfbridge.SchemaInfo{
        "vm_size": {
            ForceNew: &[]bool{true}[0],
            WarnDeleteBeforeReplace: &tfbridge.FieldDeleteBeforeReplaceWarning{
                Enabled: true,
                Reason: "VM size changes require replacement and may conflict with attached managed disks or network interfaces",
            },
        },
        "network_interface_ids": {
            ForceNew: &[]bool{true}[0],
            WarnDeleteBeforeReplace: &tfbridge.FieldDeleteBeforeReplaceWarning{
                Enabled: true,
                Reason: "Network interface changes may conflict with subnet assignments and IP allocations",
            },
        },
    },
},
```

## Writing Effective Warning Messages

### Good Warning Reasons

```go
// ✅ Good: Specific and actionable
Reason: "Instance type changes may conflict with attached EBS volumes that cannot be moved between instance types"

// ✅ Good: Explains the technical cause
Reason: "Network interface changes require delete-before-replace because interfaces cannot be attached to multiple instances simultaneously"

// ✅ Good: Mentions specific resource types
Reason: "Machine type changes may conflict with persistent disks, network interfaces, or GPU attachments"
```

### Poor Warning Reasons

```go
// ❌ Poor: Too vague
Reason: "This field might cause problems"

// ❌ Poor: No explanation of why
Reason: "Use delete-before-replace for this field"

// ❌ Poor: Technical jargon without context
Reason: "ForceNew attribute may cause resource conflict"
```

### Warning Message Guidelines

1. **Be specific** about what resources might conflict
2. **Explain why** the conflict occurs (technical reason)
3. **Mention specific attachment types** (disks, interfaces, etc.)
4. **Keep it under 100 characters** for readability
5. **Use present tense** ("changes may conflict" not "changes will conflict")

## Testing Your Configuration

### Unit Tests

Test that your warning configuration works correctly:

```go
func TestDeleteBeforeReplaceWarnings(t *testing.T) {
    provider := &tfbridge.Provider{
        // ... provider setup ...
    }
    
    // Test that warnings appear when configured fields change
    olds := resource.PropertyMap{
        "instance_type": resource.NewStringProperty("t2.micro"),
    }
    news := resource.PropertyMap{
        "instance_type": resource.NewStringProperty("t2.small"),
    }
    
    res := &tfbridge.ResourceInfo{
        Fields: map[string]*tfbridge.SchemaInfo{
            "instance_type": {
                ForceNew: &[]bool{true}[0],
                WarnDeleteBeforeReplace: &tfbridge.FieldDeleteBeforeReplaceWarning{
                    Enabled: true,
                    Reason: "Instance type changes may conflict with attached resources",
                },
            },
        },
    }
    
    warnings := provider.CheckFieldDeleteBeforeReplaceWarnings(olds, news, res)
    
    assert.Len(t, warnings, 1)
    assert.Equal(t, "instance_type", warnings[0].Field)
    assert.Contains(t, warnings[0].Reason, "may conflict")
}
```

### Integration Tests

Test the full user experience:

```go
func TestInstanceTypeChangeShowsWarning(t *testing.T) {
    // Create a test program that changes instance_type
    program := `
name: test
runtime: yaml
resources:
  instance:
    type: aws:ec2/instance:Instance
    properties:
      instanceType: t2.small  # Changed from t2.micro
      ami: ami-12345
`
    
    // Run preview and check for warnings
    result := integration.RunPulumiPreview(t, program)
    assert.Contains(t, result.Output, "may require deleteBeforeReplace")
    assert.Contains(t, result.Output, "Instance type changes may conflict")
}
```

## Best Practices

### Configuration Best Practices

1. **Start conservatively** - Only add warnings for fields with known conflict patterns
2. **Monitor user feedback** - Add warnings based on actual user support requests
3. **Update based on cloud provider changes** - Cloud APIs evolve and conflict patterns change
4. **Document in provider README** - List which fields have delete-before-replace warnings

### Message Best Practices

1. **Test messages with real users** - Ensure they're helpful and clear
2. **Keep messages actionable** - Users should know what to do next
3. **Link to documentation** - Reference provider-specific guidance when possible
4. **Update messages based on feedback** - Iterate on clarity and usefulness

### Provider Maintenance

1. **Review warnings quarterly** - Remove warnings that are no longer needed
2. **Add warnings for new resources** - New cloud services may have new conflict patterns
3. **Monitor cloud provider documentation** - Changes in replacement behavior should update warnings
4. **Track user sentiment** - Are the warnings helpful or annoying?

## Migration from Resource-Level DeleteBeforeReplace

If your provider currently uses resource-level `DeleteBeforeReplace: true`:

### Before (Resource-Level)
```go
"aws_instance": {
    Tok: "aws:ec2/instance:Instance",
    DeleteBeforeReplace: true,  // Applied to ALL field changes
},
```

### After (Field-Level)
```go
"aws_instance": {
    Tok: "aws:ec2/instance:Instance",
    // Remove resource-level setting
    Fields: map[string]*tfbridge.SchemaInfo{
        "instance_type": {
            ForceNew: &[]bool{true}[0],
            WarnDeleteBeforeReplace: &tfbridge.FieldDeleteBeforeReplaceWarning{
                Enabled: true,
                Reason: "Instance type changes may conflict with attached resources",
            },
        },
        // Only add warnings for fields that actually need them
    },
},
```

### Migration Steps

1. **Identify problematic fields** - Which specific fields cause replacement conflicts?
2. **Remove resource-level setting** - Stop applying delete-before-replace to all changes
3. **Add field-level warnings** - Configure warnings only for problematic fields
4. **Test thoroughly** - Ensure no regression in user experience
5. **Update documentation** - Reflect the more granular approach

## Common Patterns by Resource Type

### Compute Instances
- **Instance size/type changes**: Almost always need warnings
- **Network interface changes**: Usually need warnings
- **Storage configuration**: Often needs warnings
- **Simple metadata changes**: Usually don't need warnings

### Networking Resources
- **Subnet changes**: Often need warnings due to IP conflicts
- **Security group changes**: Sometimes need warnings
- **Route table changes**: Rarely need warnings

### Storage Resources
- **Disk size changes**: Usually don't need warnings (often in-place)
- **Disk type changes**: Often need warnings
- **Encryption changes**: Usually need warnings

## Troubleshooting

### Warning Not Appearing

Check that:
1. Field is configured with `WarnDeleteBeforeReplace.Enabled: true`
2. Field value is actually changing between old and new state
3. Field has `ForceNew: true` (warnings only make sense for replacement-triggering fields)

### Too Many Warnings

If users complain about too many warnings:
1. Review which fields actually cause conflicts in practice
2. Remove warnings for fields that rarely cause issues
3. Make warning messages more specific and actionable

### Warnings for Wrong Scenarios

If warnings appear when they shouldn't:
1. Check field change detection logic
2. Verify the field actually triggers replacement
3. Consider if the warning reason is accurate

This field-level warning system helps provider developers give their users more targeted guidance about when delete-before-replace is beneficial, improving the overall user experience with complex resource replacements.